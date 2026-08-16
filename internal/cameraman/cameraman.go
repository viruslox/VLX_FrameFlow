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

	// Migrate any pre-slot schema (keyed by id) to the slot-keyed layout.
	if tableExists("cameraman_devices") && !columnExists("cameraman_devices", "slot") {
		if _, err := db.Exec("DROP TABLE cameraman_devices"); err != nil {
			fmt.Printf("Failed to migrate cameraman_devices: %v\n", err)
		}
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS cameraman_devices (
		slot TEXT PRIMARY KEY,
		cam_id TEXT,
		hw_path TEXT,
		device_type TEXT,
		status TEXT
	)`)
	if err != nil {
		fmt.Printf("Failed to create table: %v\n", err)
		return
	}

	// Boot sync: restart each running slot from its stored hardware selector.
	rows, err := db.Query("SELECT slot, cam_id FROM cameraman_devices WHERE status='running'")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var slot, camID string
			if err := rows.Scan(&slot, &camID); err == nil {
				// We don't want to block init, so run in a goroutine
				go func(camID, slot string) {
					if err := StartStream(camID, slot); err != nil {
						fmt.Printf("[Cameraman Boot] Auto-start failed %s (slot %s): %v\n", camID, slot, err)
					}
				}(camID, slot)
			}
		}
	}
}

func tableExists(name string) bool {
	if db == nil {
		return false
	}
	var n string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n)
	return err == nil
}

func columnExists(table, col string) bool {
	if db == nil {
		return false
	}
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			continue
		}
		if name == col {
			return true
		}
	}
	return false
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

	// Tier 1: exact hardware-parent match (audio card and video device resolve
	// to the same PCI/USB device node). Tier 2 (fallback): collect the other
	// cards' parents here and, if tier 1 misses, accept a single unambiguous
	// topological match — this pairs composite/hubbed capture devices whose
	// audio function enumerates one node above or below the video interface.
	type audioCard struct {
		num    string
		parent string
	}
	var cards []audioCard

	for _, f := range files {
		if !strings.HasPrefix(f.Name(), "card") {
			continue
		}
		cardSysfs, err := filepath.EvalSymlinks(filepath.Join("/sys/class/sound", f.Name(), "device"))
		if err != nil {
			continue
		}

		audioParent := getHardwareParentPath(cardSysfs)
		// Match verified if both devices share the exact same hardware parent (PCI/USB)
		if audioParent == videoParent {
			return strings.TrimPrefix(f.Name(), "card")
		}
		cards = append(cards, audioCard{num: strings.TrimPrefix(f.Name(), "card"), parent: audioParent})
	}

	// Tier 2: accept a match only when exactly one card's parent is an ancestor
	// or descendant of the video parent (composite device). More than one such
	// candidate is ambiguous, so we decline and let the caller inject silence
	// rather than risk pairing an unrelated card.
	match := ""
	for _, c := range cards {
		if c.parent == "" || c.parent == "/" {
			continue
		}
		related := strings.HasPrefix(videoParent, c.parent+"/") || strings.HasPrefix(c.parent, videoParent+"/")
		if related {
			if match != "" {
				return "" // ambiguous; do not guess
			}
			match = c.num
		}
	}
	return match
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

// CameraSpec is the parsed form of a cameraman hardware identifier (VxAy).
// The identifier stays as the descriptive hardware selector; the addressable
// stream path is the separate two-digit NN slot.
type CameraSpec struct {
	VideoID       int  // V index; meaningful only when HasVideo
	AudioID       int  // A index; meaningful only when HasAudio
	HasVideo      bool // false for audio-only sources (V0A<m>)
	HasAudio      bool // false for video-only sources (V<n>A0)
	ExplicitAudio bool // an A token was supplied (true even for A0)
}

// ParseCameraID accepts:
//
//	V<n>        video with auto-discovered hardware audio
//	V<n>A<m>    video with an explicit audio device
//	V<n>A0      video with an intentionally silent track
//	V0A<m>      audio-only; a dark-grey placeholder video is synthesized
//
// V0 alone and V0A0 are rejected (a stream must carry at least one real source).
func ParseCameraID(camID string) (CameraSpec, error) {
	reCombined := regexp.MustCompile(`^V([0-9]+)A([0-9]+)$`)
	if m := reCombined.FindStringSubmatch(camID); m != nil {
		vID, _ := strconv.Atoi(m[1])
		aID, _ := strconv.Atoi(m[2])
		if vID == 0 && aID == 0 {
			return CameraSpec{}, fmt.Errorf("V0A0 is invalid: a stream needs at least one real source")
		}
		return CameraSpec{
			VideoID:       vID,
			AudioID:       aID,
			HasVideo:      vID > 0,
			HasAudio:      aID > 0,
			ExplicitAudio: true,
		}, nil
	}

	reSingle := regexp.MustCompile(`^V([0-9]+)$`)
	if m := reSingle.FindStringSubmatch(camID); m != nil {
		vID, _ := strconv.Atoi(m[1])
		if vID == 0 {
			return CameraSpec{}, fmt.Errorf("V0 alone is invalid: audio-only needs an A token, e.g. V0A1")
		}
		return CameraSpec{VideoID: vID, HasVideo: true}, nil
	}

	return CameraSpec{}, fmt.Errorf("Invalid format. Expected V1, V1A2, V1A0, or V0A2")
}

// --- Path slot (NN) helpers ---------------------------------------------

// normalizeSlot validates a user-supplied path slot and returns it as a
// zero-padded two-digit string in the range 01..99. "00" is reserved.
func normalizeSlot(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !regexp.MustCompile(`^[0-9]{1,2}$`).MatchString(s) {
		return "", fmt.Errorf("path slot must be a number 01-99")
	}
	n, _ := strconv.Atoi(s)
	if n < 1 || n > 99 {
		return "", fmt.Errorf("path slot must be in the range 01-99")
	}
	return fmt.Sprintf("%02d", n), nil
}

// runningSlots returns the set of slots that are currently streaming.
func runningSlots() map[string]bool {
	used := make(map[string]bool)
	if db == nil {
		return used
	}
	rows, err := db.Query("SELECT slot FROM cameraman_devices WHERE status='running'")
	if err != nil {
		return used
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err == nil {
			used[s] = true
		}
	}
	return used
}

func slotIsRunning(slot string) bool {
	return runningSlots()[slot]
}

// allocateSlot returns the lowest free two-digit slot among running streams.
func allocateSlot() (string, error) {
	used := runningSlots()
	for i := 1; i <= 99; i++ {
		s := fmt.Sprintf("%02d", i)
		if !used[s] {
			return s, nil
		}
	}
	return "", fmt.Errorf("no free path slots (01-99 all in use)")
}

// deviceInUse reports whether a hardware path token is already held by a
// running slot other than the one being (re)started.
func deviceInUse(hwToken, exceptSlot string) bool {
	if db == nil || hwToken == "" {
		return false
	}
	rows, err := db.Query("SELECT slot, hw_path FROM cameraman_devices WHERE status='running'")
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var slot, hwPath string
		if err := rows.Scan(&slot, &hwPath); err != nil {
			continue
		}
		if slot == exceptSlot {
			continue
		}
		for _, tok := range strings.Split(hwPath, ",") {
			if strings.TrimSpace(tok) == hwToken {
				return true
			}
		}
	}
	return false
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

// BuildStreamTarget sets the publish path of an SRT URL to an exact name
// (e.g. "cameraman_01"), preserving any publish:/read: verb and user:pass in
// the streamid. Unlike AppendCameraID it replaces rather than appends, so the
// remote MediaMTX/VisionBridge path is the stable NN slot.
func BuildStreamTarget(srtURL, pathName string) string {
	u, err := url.Parse(srtURL)
	if err != nil {
		return fmt.Sprintf("%s/%s", srtURL, pathName)
	}

	q := u.Query()
	streamid := q.Get("streamid")
	if streamid != "" {
		parts := strings.Split(streamid, ":")
		// parts[0]=publish|read, parts[1]=path, parts[2:]=user:pass
		if len(parts) >= 2 {
			parts[1] = pathName
		} else {
			parts = append(parts, pathName)
		}
		q.Set("streamid", strings.Join(parts, ":"))
		u.RawQuery = q.Encode()
		res := u.String()
		res = strings.ReplaceAll(res, "%3A", ":")
		return res
	}

	u.Path = "/" + pathName
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

func StartStream(cameraID, slot string) error {
	spec, err := ParseCameraID(cameraID)
	if err != nil {
		return err
	}

	// Resolve the two-digit path slot (NN). Empty => auto-assign the lowest
	// free slot among running streams; explicit => validate + reject overlap.
	if slot == "" {
		slot, err = allocateSlot()
		if err != nil {
			return err
		}
	} else {
		slot, err = normalizeSlot(slot)
		if err != nil {
			return err
		}
		if slotIsRunning(slot) {
			return fmt.Errorf("path slot %s is already streaming", slot)
		}
	}

	settings := LoadSettings()
	srtURL := settings["SRT_URL"]
	if srtURL == "" {
		srtURL = os.Getenv("SRT_URL")
	}

	pathPrefix := strings.Trim(settings["CAM_PATH_PREFIX"], "/")
	if pathPrefix == "" {
		pathPrefix = "cameraman"
	}

	maxResWidth, maxResHeight := parseResolution(settings["CAM_MAX_RESOLUTION"])
	maxFPS := parseFPS(settings["CAM_MAX_FPS"])
	srtURL = PrepareStreamURL(srtURL)

	payloadData := map[string]interface{}{
		"source":           "publisher",
		"runOnInitRestart": true,
	}

	// 1. Video Mapping.
	// A real V4L2 capture device when one is selected; otherwise (audio-only,
	// e.g. V0A2) a fixed dark-grey placeholder so every path carries a video
	// track and stays compatible with VisionBridge z-layers.
	var hwPathVideo string
	var videoInput, videoCodec string

	if spec.HasVideo {
		hwPathVideo, err = GetVideoDevice(spec.VideoID)
		if err != nil {
			return fmt.Errorf("Failed to get video device V%d: %w", spec.VideoID, err)
		}

		v4l2Caps := GetDeviceCapabilitiesV4L2(hwPathVideo)
		ffmpegCaps := GetDeviceCapabilitiesFFmpeg(hwPathVideo)
		bestFormat := DetermineBestFormat(v4l2Caps, ffmpegCaps, maxResWidth, maxResHeight, maxFPS)

		fps := bestFormat.FPS
		if fps == 0 {
			fps = maxFPS
		}

		formatName := strings.ToLower(bestFormat.Format)

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
	} else {
		// Audio-only: synthesize a fixed dark-grey placeholder video.
		videoInput = fmt.Sprintf(`-f lavfi -i color=c=0x2E2E2E:s=%dx%d:r=15`, maxResWidth, maxResHeight)
		videoCodec = `-c:v libx264 -preset ultrafast -tune zerolatency -pix_fmt yuv420p -g 30`
	}

	// 2. Audio Mapping & Fallback Strategy.
	//   - audio-only or explicit A<m>: take the A token as-is (A0 => silent).
	//   - bare V<n>: auto-discover the matching ALSA card via the sysfs tree.
	// Any path without real audio still gets a silent track so MediaMTX and
	// VisionBridge always see a well-formed A/V stream.
	var hwPathAudio string
	if !spec.HasVideo || spec.ExplicitAudio {
		if spec.AudioID > 0 {
			hwPathAudio, err = GetAudioDevice(spec.AudioID)
			if err != nil {
				return fmt.Errorf("Failed to get audio device A%d: %w", spec.AudioID, err)
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
		// Silent fallback (also the intentional A0 case).
		audioInput = `-f lavfi -i anullsrc=channel_layout=stereo:sample_rate=44100`
		audioCodec = `-c:a aac -b:a 128k`
	}

	// Guard against opening a capture device another running slot already holds.
	if hwPathVideo != "" && deviceInUse(hwPathVideo, slot) {
		return fmt.Errorf("video device %s is already in use by another running slot", hwPathVideo)
	}
	if hwPathAudio != "" && deviceInUse("hw:"+hwPathAudio, slot) {
		return fmt.Errorf("audio device hw:%s is already in use by another running slot", hwPathAudio)
	}

	// 3. Command Construction.
	// Remote publish path is <CAM_PATH_PREFIX>_<NN> (e.g. cameraman_01) so
	// rules in the remote MediaMTX and VisionBridge key off a stable slot.
	remotePath := fmt.Sprintf("%s_%s", pathPrefix, slot)
	ffmpegStr := fmt.Sprintf(`ffmpeg %s %s %s %s -f mpegts "%s"`, videoInput, audioInput, videoCodec, audioCodec, BuildStreamTarget(srtURL, remotePath))
	payloadData["runOnInit"] = ffmpegStr

	payloadBytes, err := json.Marshal(payloadData)
	if err != nil {
		return fmt.Errorf("Failed to marshal JSON payload: %w", err)
	}

	// 4. API Request.
	// Both local and remote paths use underscore joining (cameraman_<NN>),
	// sidestepping the path-name URL-encoding pitfalls of the v3 config API.
	apiURL := fmt.Sprintf("http://127.0.0.1:9997/v3/config/paths/add/cameraman_%s", slot)
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

	// 5. Database Update (keyed by the NN path slot).
	if db != nil {
		hwPaths := hwPathVideo
		if hwPaths == "" {
			hwPaths = "dummyvideo"
		}
		if hwPathAudio != "" {
			hwPaths += fmt.Sprintf(",hw:%s", hwPathAudio)
		} else {
			hwPaths += ",anullsrc"
		}

		devType := "AV"
		if !spec.HasVideo {
			devType = "AO"
		} else if hwPathAudio == "" {
			devType = "VO"
		}

		_, err = db.Exec(
			"INSERT INTO cameraman_devices (slot, cam_id, hw_path, device_type, status) VALUES (?, ?, ?, ?, 'running') ON CONFLICT(slot) DO UPDATE SET cam_id=?, hw_path=?, device_type=?, status='running'",
			slot, cameraID, hwPaths, devType, cameraID, hwPaths, devType,
		)
		if err != nil {
			fmt.Printf("Failed to update database: %v\n", err)
		}
	}

	return nil
}

func StopStream(slot string) error {
	slot, err := normalizeSlot(slot)
	if err != nil {
		return err
	}
	apiURL := fmt.Sprintf("http://127.0.0.1:9997/v3/config/paths/delete/cameraman_%s", slot)

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
		_, err = db.Exec("UPDATE cameraman_devices SET status='stopped' WHERE slot=?", slot)
		if err != nil {
			fmt.Printf("Failed to update database: %v\n", err)
		}
	}

	return nil
}

func StatusStream(slot string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("Database not initialized")
	}

	slot, err := normalizeSlot(slot)
	if err != nil {
		return "", err
	}

	var status, camID string
	err = db.QueryRow("SELECT status, cam_id FROM cameraman_devices WHERE slot=?", slot).Scan(&status, &camID)
	if err != nil {
		return "", fmt.Errorf("Slot %s not found in DB", slot)
	}

	label := fmt.Sprintf("%s (%s)", slot, camID)

	apiURL := fmt.Sprintf("http://127.0.0.1:9997/v3/paths/get/cameraman_%s", slot)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Sprintf("● %s - API: not found, DB: %s", label, status), nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("● %s - API: not found, DB: %s", label, status), nil
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		return fmt.Sprintf("● %s - API: not found, DB: %s", label, status), nil
	}

	return fmt.Sprintf("● %s - API: active, DB: %s", label, status), nil
}

func StatusAllStreams() (string, error) {
	if db == nil {
		return "No active cameraman services running.", nil
	}

	rows, err := db.Query("SELECT slot, cam_id, status FROM cameraman_devices WHERE status='running'")
	if err != nil {
		return "No active cameraman services running.", nil
	}
	defer rows.Close()

	var activeUnits []string
	for rows.Next() {
		var slot, camID, status string
		if err := rows.Scan(&slot, &camID, &status); err == nil {
			activeUnits = append(activeUnits, fmt.Sprintf("● %s (%s) - %s", slot, camID, status))
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
	b.WriteString(" Tip: 'V1' auto-discovers hardware audio; 'V1A2' forces V1 with A2.\n")
	b.WriteString("      'V1A0' = video with a silent track; 'V0A2' = audio-only with a\n")
	b.WriteString("      dark-grey placeholder video. The two-digit NN sets the path:\n")
	b.WriteString("      e.g. 'cameraman V1 01 start' publishes to <CAM_PATH_PREFIX>_01.\n")
	b.WriteString("      Omit NN to auto-assign the lowest free slot.\n")
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
