package cameraman

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

var (
	pingFailCount int
	pingMutex     sync.Mutex
)

func GetVideoDevice(vidID int) (string, error) {
	if vidID == 0 {
		return "", nil
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
			isValid = true
			if ignoreRegex.MatchString(line) {
				isValid = false
			}
			continue
		}

		if isValid && strings.Contains(line, "/dev/video") {
			path := strings.TrimSpace(line)
			validDevices = append(validDevices, path)
			// Wait, the bash script says `if ($1 ~ /\/dev\/video/) { print $1; printed_path = 1 }`
			// and `!printed_path` so it only prints the first /dev/video* per device group.
			// Let's correctly implement it.
			isValid = false // Set to false to avoid printing other paths for the same device
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
			// fields[1] is something like "0:"
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

func ParseCameraID(camID string) (int, int, error) {
	re := regexp.MustCompile(`^V([0-9]+)A([0-9]+)$`)
	matches := re.FindStringSubmatch(camID)

	if matches == nil {
		return 0, 0, fmt.Errorf("Invalid format. Expected VxAy")
	}

	vidID, _ := strconv.Atoi(matches[1])
	audID, _ := strconv.Atoi(matches[2])

	if vidID == 0 && audID == 0 {
		return 0, 0, fmt.Errorf("Both Video and Audio ID cannot be 0.")
	}

	return vidID, audID, nil
}

func BuildStreamURL(baseURL, mode string, vidID, audID int) string {
	finalURL := baseURL
	urlSuffix := fmt.Sprintf("%d", vidID)

	if vidID == 0 {
		urlSuffix = fmt.Sprintf("A%d", audID)
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		fmt.Printf("Error: Base stream URL is not parsable: %v\n", err)
		return baseURL
	}

	if mode == "rtsp" {
		u.Path = fmt.Sprintf("%s_%s", u.Path, urlSuffix)
		finalURL = u.String()
	} else if mode == "mpegts" {
		streamID := u.Query().Get("streamid")
		if streamID != "" && strings.HasPrefix(streamID, "publish:") {
			parts := strings.SplitN(streamID, "publish:", 2)
			if len(parts) == 2 {
				content := parts[1]
				if strings.Contains(content, ":") {
					contentParts := strings.SplitN(content, ":", 2)
					streamName := contentParts[0]
					authData := contentParts[1]
					newStreamID := fmt.Sprintf("publish:%s_%s:%s", streamName, urlSuffix, authData)
					finalURL = strings.Replace(baseURL, "streamid="+streamID, "streamid="+newStreamID, 1)
				} else {
					newStreamID := fmt.Sprintf("publish:%s_%s", content, urlSuffix)
					finalURL = strings.Replace(baseURL, "streamid="+streamID, "streamid="+newStreamID, 1)
				}
			}
		} else {
			u.Path = fmt.Sprintf("%s_%s", u.Path, urlSuffix)
			finalURL = u.String()
		}
	}

	if _, err := url.ParseRequestURI(finalURL); err != nil {
		fmt.Printf("Error: Generated stream URL is invalid: %v\n", err)
	}

	return finalURL
}

func PrepareStreamURL(protocol, rtspURL, srtURL, role string) (string, string, error) {
	switch protocol {
	case "rtsp":
		if rtspURL == "" {
			return "", "", fmt.Errorf("RTSP_URL not set in profile")
		}
		return rtspURL, "rtsp", nil
	default: // srt or others
		if srtURL == "" {
			return "", "", fmt.Errorf("SRT_URL not set in profile")
		}
		strURL := srtURL
		if strings.ToUpper(role) == "CLIENT" {
			// ping -c 1 -W 2 10.1.10.1
			_, err := sysutils.RunCommand(2*time.Second, "ping", "-c", "1", "-W", "2", "10.1.10.1")

			pingMutex.Lock()
			if err == nil {
				pingFailCount = 0
			} else {
				pingFailCount++
			}
			failCount := pingFailCount
			pingMutex.Unlock()

			if failCount < 3 {
				// Rewrite srt://something to srt://10.1.10.1
				// regex replace srt://[^/:]+
				re := regexp.MustCompile(`srt://[^/:]+`)
				strURL = re.ReplaceAllString(srtURL, "srt://10.1.10.1")
			}
		}
		return strURL, "mpegts", nil
	}
}

func StartStream(cameraID string, vidID, audID int) error {
	settings := LoadSettings()
	ffmpegPath := "/usr/bin/ffmpeg"
	ffmpegOut, _ := sysutils.RunCommand(2*time.Second, "which", "ffmpeg")
	if ffmpegOut != "" {
		ffmpegPath = strings.TrimSpace(ffmpegOut)
	}

	rtspURL := settings["RTSP_URL"]
	if rtspURL == "" {
		rtspURL = os.Getenv("RTSP_URL")
	}

	srtURL := settings["SRT_URL"]
	if srtURL == "" {
		srtURL = os.Getenv("SRT_URL")
	}

	protocol := settings["STREAM_PROTOCOL"]
	if protocol == "" {
		protocol = os.Getenv("STREAM_PROTOCOL")
	}
	if protocol == "" {
		protocol = "srt"
	}

	role := settings["FRAMEFLOW_ROLE"]
	if role == "" {
		role = os.Getenv("FRAMEFLOW_ROLE")
	}
	if role == "" {
		role = "CLIENT"
	}

	strURL, strMode, err := PrepareStreamURL(protocol, rtspURL, srtURL, role)
	if err != nil {
		return err
	}
	unitName := fmt.Sprintf("frameflow-stream-%s", cameraID)

	_, err = sysutils.RunCommand(5*time.Second, "systemctl", "--user", "is-active", "--quiet", unitName)
	if err == nil {
		return fmt.Errorf("Service %s is already running.", unitName)
	}

	videoDevice, err := GetVideoDevice(vidID)
	if err != nil {
		return err
	}

	audioDeviceHW, err := GetAudioDevice(audID)
	if err != nil {
		return err
	}

	syncOpts := []string{"-thread_queue_size", "2048", "-use_wallclock_as_timestamps", "1", "-fflags", "+genpts"}
	var cmd []string
	cmd = append(cmd, ffmpegPath)

	if vidID != 0 {
		cmd = append(cmd, syncOpts...)
		cmd = append(cmd, "-f", "v4l2", "-framerate", "30", "-video_size", "1920x1080", "-i", videoDevice)
	}

	if audioDeviceHW != "" {
		cmd = append(cmd, syncOpts...)
		cmd = append(cmd, "-f", "alsa", "-i", fmt.Sprintf("hw:%s", audioDeviceHW), "-c:a", "aac", "-b:a", "128k", "-af", "aresample=async=1")
	}

	finalURL := BuildStreamURL(strURL, strMode, vidID, audID)

	if vidID != 0 {
		cmd = append(cmd, "-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency")
	}
	cmd = append(cmd, "-f", strMode, finalURL)

	runArgs := []string{
		"--user",
		fmt.Sprintf("--unit=%s", unitName),
		fmt.Sprintf("--description=VLX FrameFlow Stream %s", cameraID),
		"--collect",
		"--property=Restart=on-failure",
		"--property=RestartSec=5",
		"--service-type=exec",
	}
	runArgs = append(runArgs, cmd...)

	_, err = sysutils.RunCommand(10*time.Second, "systemd-run", runArgs...)
	if err != nil {
		return fmt.Errorf("failed to start stream unit: %w", err)
	}

	time.Sleep(2 * time.Second)

	_, err = sysutils.RunCommand(5*time.Second, "systemctl", "--user", "is-active", "--quiet", unitName)
	if err == nil {
		return nil
	}

	return fmt.Errorf("Failed to start stream. Check logs.")
}

func StopStream(cameraID string) error {
	unitName := fmt.Sprintf("frameflow-stream-%s", cameraID)
	_, err := sysutils.RunCommand(10*time.Second, "systemctl", "--user", "stop", unitName)
	if err != nil {
		return err
	}
	return nil
}

func StatusStream(cameraID string) (string, error) {
	unitName := fmt.Sprintf("frameflow-stream-%s", cameraID)
	out, err := sysutils.RunCommand(10*time.Second, "systemctl", "--user", "status", unitName, "--no-pager")
	return out, err
}

func StatusAllStreams() (string, error) {
	out, err := sysutils.RunCommand(10*time.Second, "systemctl", "--user", "list-units", "frameflow-stream-*", "--no-legend", "--plain")
	if err != nil {
		return "", err
	}

	lines := strings.Split(out, "\n")
	var activeUnits []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			desc := strings.Join(fields[4:], " ")
			activeUnits = append(activeUnits, "● "+fields[0]+" - "+desc)
		} else if len(fields) > 0 {
			activeUnits = append(activeUnits, "● "+fields[0])
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
				isValid = true
				if ignoreRegex.MatchString(line) {
					isValid = false
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
	b.WriteString("  A0   | none   | No audio (Silent)\n")
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
