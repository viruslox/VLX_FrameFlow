package cameraman

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"
	_ "modernc.org/sqlite"
)

var (
	pingFailCount int
	pingMutex     sync.Mutex
	db            *sql.DB
)

func init() {
	settings := LoadSettings()
	dsn := settings["DB_DSN"]
	if dsn == "" {
		dsn = os.Getenv("DB_DSN")
	}
	if dsn == "" {
		dsn = "/opt/VLX_FrameFlow/var/frameflow.db"
	}

	var err error

	// Create directory if it doesn't exist
	dbDir := filepath.Dir(dsn)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		fmt.Printf("Failed to create database directory: %v\n", err)
	}

	dbDSN := dsn
	if strings.Contains(dbDSN, "?") {
		dbDSN += "&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	} else {
		dbDSN += "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	}

	db, err = sql.Open("sqlite", dbDSN)
	if err != nil {
		fmt.Printf("Failed to open database: %v\n", err)
		return
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS cameraman_devices (
		id TEXT PRIMARY KEY,
		hw_path TEXT,
		device_type TEXT,
		status TEXT
	)`)
	if err != nil {
		fmt.Printf("Failed to create table: %v\n", err)
		return
	}

	// Boot sync
	rows, err := db.Query("SELECT id FROM cameraman_devices WHERE status='running'")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				// We don't want to block init, so run in a goroutine
				go func(camID string) {
					if err := StartStream(camID); err != nil {
						fmt.Printf("[Cameraman Boot] Auto-start failed %s: %v\n", camID, err)
					}
				}(id)
			}
		}
	}
}

func GetVideoDevice(vidID int) (string, error) {
	if vidID == 0 {
		return "", fmt.Errorf("Video device V0 is invalid")
	}

	out, err := sysutils.RunCommand(10*time.Second, "v4l2-ctl", "--list-devices")
	if err != nil {
		return "", fmt.Errorf("v4l2-ctl failed: %w", err)
	}

	var validDevices []string
	ignoreRegex := regexp.MustCompile(`(codec|enc|dec|vpu|rga|isp|platform:.*-video)`)

	lines := strings.Split(out, "\n")
	isValid := false

	for _, line := range lines {
		// New device group starts without leading space/tab
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			isValid = false
			if !ignoreRegex.MatchString(line) {
				isValid = true
			}
			continue
		}

		if isValid && strings.Contains(line, "/dev/video") {
			path := strings.TrimSpace(line)
			validDevices = append(validDevices, path)
			// Avoid printing other paths for the same device group
			isValid = false
		}
	}

	index := vidID - 1
	if index < 0 || index >= len(validDevices) {
		return "", fmt.Errorf("Video device V%d not found", vidID)
	}

	return validDevices[index], nil
}

func GetAudioDevice(audID int) (string, error) {
	if audID == 0 {
		return "", nil
	}

	out, err := sysutils.RunCommand(10*time.Second, "arecord", "-l")
	if err != nil {
		return "", fmt.Errorf("arecord failed: %w", err)
	}

	var audioDevices []string
	lines := strings.Split(out, "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "card" {
			// Extract card ID
			id := strings.TrimSuffix(fields[1], ":")
			audioDevices = append(audioDevices, id)
		}
	}

	index := audID - 1
	if index < 0 || index >= len(audioDevices) {
		return "", fmt.Errorf("Audio device A%d not found", audID)
	}

	return audioDevices[index], nil
}

// getHardwareParentPath traverses sysfs upwards to find the parent USB or PCI device
func getHardwareParentPath(sysfsPath string) string {
	current := sysfsPath
	for current != "/" && current != "." && len(current) > 4 {
		hasUSB := false
		if _, err := os.Stat(filepath.Join(current, "idVendor")); err == nil {
			hasUSB = true
		}
		
		hasPCI := false
		if _, err := os.Stat(filepath.Join(current, "vendor")); err == nil && !hasUSB {
			if _, err := os.Stat(filepath.Join(current, "device")); err == nil {
				hasPCI = true
			}
		}
		
		if hasUSB || hasPCI {
			return current
		}
		current = filepath.Dir(current)
	}
	// Fallback to direct parent if vendor signatures are missing
	return filepath.Dir(sysfsPath)
}

// FindMatchingAudioDevice attempts to auto-discover the associated ALSA card via sysfs topology
func FindMatchingAudioDevice(videoDevPath string) string {
	videoDevName := filepath.Base(videoDevPath)
	videoSysfs, err := filepath.EvalSymlinks(fmt.Sprintf("/sys/class/video4linux/%s/device", videoDevName))
	if err != nil {
		return ""
	}
	
	videoParent := getHardwareParentPath(videoSysfs)
	if videoParent == "" || videoParent == "/" {
		return ""
	}

	files, err := os.ReadDir("/sys/class/sound")
	if err != nil {
		return ""
	}

	for _, f := range files {
		if strings.HasPrefix(f.Name(), "card") {
			cardSysfs, err := filepath.EvalSymlinks(filepath.Join("/sys/class/sound", f.Name(), "device"))
			if err != nil {
				continue
			}
			
			audioParent := getHardwareParentPath(cardSysfs)
			// Match verified if both devices share the exact same hardware parent (PCI/USB)
			if audioParent == videoParent {
				return strings.TrimPrefix(f.Name(), "card")
			}
		}
	}
	return ""
}

type FormatCapability struct {
	Format string
	Width  int
	Height int
	FPS    float64
}

func parseV4l2Ctl(out string) []FormatCapability {
	var caps []FormatCapability
	lines := strings.Split(out, "\n")

	var currentFormat string
	var currentWidth, currentHeight int

	formatRe := regexp.MustCompile(`\[\d+\]: '([^']+)'`)
	sizeRe := regexp.MustCompile(`Size: Discrete (\d+)x(\d+)`)
	fpsRe := regexp.MustCompile(`Interval: Discrete .* \(([\d\.]+) fps\)`)

	for _, line := range lines {
		if m := formatRe.FindStringSubmatch(line); m != nil {
			currentFormat = m[1]
			currentWidth = 0
			currentHeight = 0
		} else if m := sizeRe.FindStringSubmatch(line); m != nil {
			fmt.Sscanf(m[1], "%d", &currentWidth)
			fmt.Sscanf(m[2], "%d", &currentHeight)
		} else if m := fpsRe.FindStringSubmatch(line); m != nil {
			var fps float64
			fmt.Sscanf(m[1], "%f", &fps)
			if currentFormat != "" && currentWidth > 0 && currentHeight > 0 {
				caps = append(caps, FormatCapability{
					Format: currentFormat,
					Width:  currentWidth,
					Height: currentHeight,
					FPS:    fps,
				})
			}
		}
	}
	return caps
}

func parseFfmpegFormats(out string) []FormatCapability {
	var caps []FormatCapability
	lines := strings.Split(out, "\n")

	re := regexp.MustCompile(`^\[.*?\]\s*(?:Raw|Compressed)\s*:\s*(.*?)\s*:\s*(.*?)\s*:\s*(.*)$`)

	for _, line := range lines {
		m := re.FindStringSubmatch(strings.TrimSpace(line))
		if m != nil {
			formatName := strings.TrimSpace(m[1])
			resolutionsStr := strings.TrimSpace(m[3])

			resolutions := strings.Fields(resolutionsStr)
			for _, res := range resolutions {
				var w, h int
				if n, err := fmt.Sscanf(res, "%dx%d", &w, &h); err == nil && n == 2 {
					caps = append(caps, FormatCapability{
						Format: formatName,
						Width:  w,
						Height: h,
						FPS:    30.0,
					})
				}
			}
		}
	}
	return caps
}

func DetermineBestFormat(v4l2Caps, ffmpegCaps []FormatCapability, maxResWidth, maxResHeight int, maxFPS float64) FormatCapability {
	var caps []FormatCapability
	if len(v4l2Caps) > 0 {
		caps = v4l2Caps
	} else if len(ffmpegCaps) > 0 {
		caps = ffmpegCaps
	} else {
		return FormatCapability{Format: "default", Width: maxResWidth, Height: maxResHeight, FPS: maxFPS}
	}

	formatScore := func(f string) int {
		f = strings.ToLower(f)
		if strings.Contains(f, "h264") {
			return 100
		}
		if strings.Contains(f, "mjpg") || strings.Contains(f, "mjpeg") {
			return 50
		}
		return 10
	}

	bestScore := -1.0
	var bestCap FormatCapability
	foundMatch := false

	for _, cap := range caps {
		if cap.Width > maxResWidth || cap.Height > maxResHeight || cap.FPS > maxFPS {
			continue
		}

		score := float64(formatScore(cap.Format)*10000 + cap.Width*cap.Height)
		if score > bestScore {
			bestScore = score
			bestCap = cap
			foundMatch = true
		}
	}

	// If all caps exceed the user limit, pick the one closest to the limit
	if !foundMatch {
		minDiff := 1e12
		for _, cap := range caps {
			diff := math.Abs(float64(cap.Width-maxResWidth)) + math.Abs(float64(cap.Height-maxResHeight))
			diffScore := diff - float64(formatScore(cap.Format)*1000)

			if diffScore < minDiff {
				minDiff = diffScore
				bestCap = cap
			}
		}
	}

	return bestCap
}

func GetDeviceCapabilitiesV4L2(hwPath string) []FormatCapability {
	out, err := sysutils.RunCommand(10*time.Second, "v4l2-ctl", "-d", hwPath, "--list-formats-ext")
	if err != nil {
		return nil
	}
	return parseV4l2Ctl(out)
}

func GetDeviceCapabilitiesFFmpeg(hwPath string) []FormatCapability {
	out, err := sysutils.RunCommand(10*time.Second, "ffmpeg", "-f", "v4l2", "-list_formats", "all", "-i", hwPath)
	if err != nil {
		return parseFfmpegFormats(out)
	}
	return parseFfmpegFormats(out)
}

// ParseCameraID handles formats like V1 (auto audio) or V1A2 (manual override)
// Returns: videoID, audioID, hasExplicitAudio, error
func ParseCameraID(camID string) (int, int, bool, error) {
	reCombined := regexp.MustCompile(`^V([0-9]+)A([0-9]+)$`)
	if matches := reCombined.FindStringSubmatch(camID); matches != nil {
		vID, _ := strconv.Atoi(matches[1])
		aID, _ := strconv.Atoi(matches[2])
		if vID == 0 {
			return 0, 0, false, fmt.Errorf("Video device ID cannot be 0")
		}
		return vID, aID, true, nil
	}

	reSingle := regexp.MustCompile(`^V([0-9]+)$`)
	if matches := reSingle.FindStringSubmatch(camID); matches != nil {
		vID, _ := strconv.Atoi(matches[1])
		if vID == 0 {
			return 0, 0, false, fmt.Errorf("Video device ID cannot be 0")
		}
		return vID, 0, false, nil
	}

	return 0, 0, false, fmt.Errorf("Invalid format. Expected V1, V1A2, etc.")
}

func PrepareStreamURL(srtURL string) string {
	finalSRT := srtURL

	// Check if bonding server is reachable
	bondingServerReach := false
	_, err := sysutils.RunCommand(2*time.Second, "ping", "-c", "1", "-W", "1", "10.1.10.1")
	if err == nil {
		bondingServerReach = true
	}

	if bondingServerReach {
		re := regexp.MustCompile(`srt://[^/:]+`)
		finalSRT = re.ReplaceAllString(srtURL, "srt://10.1.10.1")
	}

	return finalSRT
}

func AppendCameraID(srtURL, cameraID string) string {
	u, err := url.Parse(srtURL)
	if err != nil {
		return fmt.Sprintf("%s_%s", srtURL, cameraID)
	}

	q := u.Query()
	streamid := q.Get("streamid")

	if streamid != "" {
		parts := strings.Split(streamid, ":")
		if len(parts) >= 2 {
			parts[1] = fmt.Sprintf("%s_%s", parts[1], cameraID)
		} else {
			parts[0] = fmt.Sprintf("%s_%s", parts[0], cameraID)
		}
		q.Set("streamid", strings.Join(parts, ":"))
		u.RawQuery = q.Encode()
		res := u.String()
		res = strings.ReplaceAll(res, "%3A", ":")
		return res
	}

	if u.Path != "" {
		u.Path = fmt.Sprintf("%s_%s", u.Path, cameraID)
	} else {
		u.Path = fmt.Sprintf("/_%s", cameraID)
	}

	res := u.String()
	res = strings.ReplaceAll(res, "%3A", ":")
	return res
}

func parseResolution(res string) (int, int) {
	if res == "" {
		return 1920, 1080
	}
	parts := strings.Split(res, "x")
	if len(parts) == 2 {
		w, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		h, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		if w > 0 && h > 0 {
			return w, h
		}
	}
	return 1920, 1080
}

func parseFPS(fpsStr string) float64 {
	if fpsStr == "" {
		return 30.0
	}
	fps, err := strconv.ParseFloat(strings.TrimSpace(fpsStr), 64)
	if err != nil || fps <= 0 {
		return 30.0
	}
	return fps
}

func StartStream(cameraID string) error {
	vID, explicitAID, explicitAudio, err := ParseCameraID(cameraID)
	if err != nil {
		return err
	}

	settings := LoadSettings()
	srtURL := settings["SRT_URL"]
	if srtURL == "" {
		srtURL = os.Getenv("SRT_URL")
	}

	maxResWidth, maxResHeight := parseResolution(settings["CAM_MAX_RESOLUTION"])
	maxFPS := parseFPS(settings["CAM_MAX_FPS"])
	srtURL = PrepareStreamURL(srtURL)

	payloadData := map[string]interface{}{
		"source":           "publisher",
		"runOnInitRestart": true,
	}

	// 1. Video Hardware Mapping
	hwPathVideo, err := GetVideoDevice(vID)
	if err != nil {
		return fmt.Errorf("Failed to get video device V%d: %w", vID, err)
	}

	v4l2Caps := GetDeviceCapabilitiesV4L2(hwPathVideo)
	ffmpegCaps := GetDeviceCapabilitiesFFmpeg(hwPathVideo)
	bestFormat := DetermineBestFormat(v4l2Caps, ffmpegCaps, maxResWidth, maxResHeight, maxFPS)

	fps := bestFormat.FPS
	if fps == 0 {
		fps = maxFPS
	}

	formatName := strings.ToLower(bestFormat.Format)
	var videoInput, videoCodec string

	if formatName == "default" {
		videoInput = fmt.Sprintf(`-f v4l2 -framerate %f -video_size %dx%d -i %s`, fps, bestFormat.Width, bestFormat.Height, hwPathVideo)
		videoCodec = `-c:v libx264 -preset ultrafast -tune zerolatency`
	} else if strings.Contains(formatName, "h264") {
		videoInput = fmt.Sprintf(`-f v4l2 -input_format h264 -framerate %f -video_size %dx%d -i %s`, fps, bestFormat.Width, bestFormat.Height, hwPathVideo)
		videoCodec = `-c:v copy`
	} else if strings.Contains(formatName, "mjpg") || strings.Contains(formatName, "mjpeg") {
		videoInput = fmt.Sprintf(`-f v4l2 -input_format mjpeg -framerate %f -video_size %dx%d -i %s`, fps, bestFormat.Width, bestFormat.Height, hwPathVideo)
		videoCodec = `-c:v libx264 -preset ultrafast -tune zerolatency`
	} else {
		videoInput = fmt.Sprintf(`-f v4l2 -input_format %s -framerate %f -video_size %dx%d -i %s`, formatName, fps, bestFormat.Width, bestFormat.Height, hwPathVideo)
		videoCodec = `-c:v libx264 -preset ultrafast -tune zerolatency`
	}

	// 2. Audio Hardware Mapping & Fallback Strategy
	var hwPathAudio string
	if explicitAudio {
		if explicitAID > 0 {
			hwPathAudio, err = GetAudioDevice(explicitAID)
			if err != nil {
				return fmt.Errorf("Failed to get audio device A%d: %w", explicitAID, err)
			}
		}
	} else {
		// Auto-discovery via sysfs hardware tree
		hwPathAudio = FindMatchingAudioDevice(hwPathVideo)
	}

	var audioInput, audioCodec string
	if hwPathAudio != "" {
		audioInput = fmt.Sprintf(`-f alsa -i hw:%s`, hwPathAudio)
		audioCodec = `-c:a aac -b:a 128k -af aresample=async=1`
	} else {
		// Essential for MediaMTX: Always generate a silent audio track if hardware is missing
		audioInput = `-f lavfi -i anullsrc=channel_layout=stereo:sample_rate=44100`
		audioCodec = `-c:a aac -b:a 128k`
	}

	// 3. Command Construction
	ffmpegStr := fmt.Sprintf(`ffmpeg %s %s %s %s -f mpegts "%s"`, videoInput, audioInput, videoCodec, audioCodec, AppendCameraID(srtURL, cameraID))
	payloadData["runOnInit"] = ffmpegStr

	payloadBytes, err := json.Marshal(payloadData)
	if err != nil {
		return fmt.Errorf("Failed to marshal JSON payload: %w", err)
	}

	// 4. API Request
	apiURL := fmt.Sprintf("http://127.0.0.1:9997/v3/config/paths/add/cameraman_%s", cameraID)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("Failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to configure MediaMTX API: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("MediaMTX API returned status: %d", resp.StatusCode)
	}

	// 5. Database Update
	if db != nil {
		hwPaths := hwPathVideo
		if hwPathAudio != "" {
			hwPaths += fmt.Sprintf(",hw:%s", hwPathAudio)
		} else {
			hwPaths += ",anullsrc"
		}
		
		_, err = db.Exec("INSERT INTO cameraman_devices (id, hw_path, device_type, status) VALUES (?, ?, 'AV', 'running') ON CONFLICT(id) DO UPDATE SET status='running', hw_path=?, device_type='AV'", cameraID, hwPaths, hwPaths)
		if err != nil {
			fmt.Printf("Failed to update database: %v\n", err)
		}
	}

	return nil
}

func StopStream(cameraID string) error {
	apiURL := fmt.Sprintf("http://127.0.0.1:9997/v3/config/paths/delete/cameraman_%s", cameraID)

	req, err := http.NewRequest("DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("Failed to create HTTP request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to delete from MediaMTX API: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("MediaMTX API returned status: %d", resp.StatusCode)
	}

	if db != nil {
		_, err = db.Exec("UPDATE cameraman_devices SET status='stopped' WHERE id=?", cameraID)
		if err != nil {
			fmt.Printf("Failed to update database: %v\n", err)
		}
	}

	return nil
}

func StatusStream(cameraID string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("Database not initialized")
	}

	var status string
	err := db.QueryRow("SELECT status FROM cameraman_devices WHERE id=?", cameraID).Scan(&status)
	if err != nil {
		return "", fmt.Errorf("Stream %s not found in DB", cameraID)
	}

	apiURL := fmt.Sprintf("http://127.0.0.1:9997/v3/paths/get/cameraman_%s", cameraID)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Sprintf("● %s - API: not found, DB: %s", cameraID, status), nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("● %s - API: not found, DB: %s", cameraID, status), nil
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		return fmt.Sprintf("● %s - API: not found, DB: %s", cameraID, status), nil
	}

	return fmt.Sprintf("● %s - API: active, DB: %s", cameraID, status), nil
}

func StatusAllStreams() (string, error) {
	if db == nil {
		return "No active cameraman services running.", nil
	}

	rows, err := db.Query("SELECT id, status FROM cameraman_devices WHERE status='running'")
	if err != nil {
		return "No active cameraman services running.", nil
	}
	defer rows.Close()

	var activeUnits []string
	for rows.Next() {
		var id, status string
		if err := rows.Scan(&id, &status); err == nil {
			activeUnits = append(activeUnits, fmt.Sprintf("● %s - %s", id, status))
		}
	}

	if len(activeUnits) == 0 {
		return "No active cameraman services running.", nil
	}

	return strings.Join(activeUnits, "\n"), nil
}

func ListDevices() (string, error) {
	var b strings.Builder
	b.WriteString("------------------------------------------------------------------------\n")
	b.WriteString(" [VIDEO DEVICES] Mapping for Vx\n")
	b.WriteString("------------------------------------------------------------------------\n")
	out, err := sysutils.RunCommand(10*time.Second, "v4l2-ctl", "--list-devices")
	if err == nil {
		b.WriteString("  ID   | HW Path      | Device Description\n")
		b.WriteString("  -----|--------------|-------------------------------------------------\n")
		ignoreRegex := regexp.MustCompile(`(codec|enc|dec|vpu|rga|isp|platform:.*-video)`)
		lines := strings.Split(out, "\n")
		isValid := false
		var desc string
		vIdx := 1
		printedPath := false
		for _, line := range lines {
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
				isValid = false
				if !ignoreRegex.MatchString(line) {
					isValid = true
				}
				if isValid {
					desc = strings.TrimSpace(line)
					desc = strings.TrimSuffix(desc, ":")
					printedPath = false
				}
				continue
			}
			if isValid && !printedPath && strings.Contains(line, "/dev/video") {
				path := strings.TrimSpace(line)
				b.WriteString(fmt.Sprintf("  V%-3d | %-12s | %s\n", vIdx, path, desc))
				vIdx++
				printedPath = true
			}
		}
	} else {
		b.WriteString("[ERR] 'v4l2-ctl' not installed.\n")
	}
	b.WriteString("\n")
	b.WriteString("------------------------------------------------------------------------\n")
	b.WriteString(" [AUDIO DEVICES] Mapping for Ay\n")
	b.WriteString("------------------------------------------------------------------------\n")
	b.WriteString("  ID   | HW Idx | Device Description\n")
	b.WriteString("  -----|--------|---------------------------------------------------------\n")
	out, err = sysutils.RunCommand(10*time.Second, "arecord", "-l")
	if err == nil {
		aIdx := 1
		lines := strings.Split(out, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "card ") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					id := strings.TrimSuffix(fields[1], ":")
					idx := strings.Index(line, ":")
					desc := ""
					if idx != -1 && len(line) > idx+2 {
						desc = strings.TrimSpace(line[idx+2:])
					}
					b.WriteString(fmt.Sprintf("  A%-3d | %-6s | %s\n", aIdx, "hw:"+id, desc))
					aIdx++
				}
			}
		}
	} else {
		b.WriteString("[ERR] 'arecord' not installed.\n")
	}
	b.WriteString("\n")
	b.WriteString("------------------------------------------------------------------------\n")
	b.WriteString(" Tip: Start streams using 'V1' (auto-discovers hardware audio) \n")
	b.WriteString("      or use manual override like 'V1A2' (forces V1 with A2).\n")
	b.WriteString("      Silent video devices will automatically generate a blank audio track.\n")
	b.WriteString("------------------------------------------------------------------------\n")
	return b.String(), nil
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func LoadSettings() map[string]string {
	settings := make(map[string]string)

	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}
	filepaths := []string{filepath.Join(vlxSuiteDir, "etc/frameflow.settings"), "bin/frameflow.settings", "frameflow.settings"}

	var file *os.File
	var err error

	for _, path := range filepaths {
		file, err = os.Open(path)
		if err == nil {
			break
		}
	}

	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if len(line) == 0 || strings.HasPrefix(line, "#") {
				continue
			}

			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				key = strings.TrimPrefix(key, "export ")
				val := trimQuotes(strings.TrimSpace(parts[1]))
				settings[key] = val
			}
		}
	}

	return settings
}
