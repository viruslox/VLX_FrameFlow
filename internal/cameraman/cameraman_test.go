package cameraman

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		cmd := os.Args[3]
		switch cmd {
		case "v4l2-ctl":
			fmt.Print(`OBSBOT Tiny 2 Lite: OBSBOT Ti (usb-xhci-hcd.10.auto-1.1):
	/dev/media1
	/dev/video4
	/dev/video5

UGREEN-25854: UGREEN-25854 (usb-xhci-hcd.10.auto-1.3):
	/dev/media2
	/dev/video6
	/dev/video7

rkisp1_mainpath (platform:rkisp1):
	/dev/video0
	/dev/video1

rk-codec-enc (platform:rk-codec-enc):
	/dev/video2
`)
			os.Exit(0)
		case "arecord":
			fmt.Print(`**** List of CAPTURE Hardware Devices ****
card 0: RockchipRK3588 [Rockchip-RK3588], device 0: fe470000.i2s-rk3588-hifi rk3588-hifi-0 [fe470000.i2s-rk3588-hifi rk3588-hifi-0]
  Subdevices: 1/1
  Subdevice #0: subdevice #0
card 1: UGREEN25854 [UGREEN-25854], device 0: USB Audio [USB Audio]
  Subdevices: 1/1
  Subdevice #0: subdevice #0
card 3: AnotherDevice [AnotherDevice], device 0: USB Audio [USB Audio]
  Subdevices: 1/1
  Subdevice #0: subdevice #0
`)
			os.Exit(0)
		case "ping":
			if os.Getenv("MOCK_PING_FAIL") == "1" {
				os.Exit(1)
			}
			os.Exit(0)
		case "systemctl":
			args := os.Args[3:]
			if len(args) > 2 && os.Getenv("MOCK_SYSTEMCTL_IS_ACTIVE") == "1" && args[2] == "is-active" {
				// We expect unit to be empty for is-active to succeed normally
				os.Exit(0)
			}
			if len(args) > 2 && os.Getenv("MOCK_SYSTEMCTL_FAIL_IS_ACTIVE") == "1" && args[2] == "is-active" {
				os.Exit(1) // not active
			}
			if len(args) > 2 && os.Getenv("MOCK_SYSTEMCTL_FAIL_IS_ACTIVE_AFTER_START") == "1" && args[2] == "is-active" {
				if os.Getenv("IS_ACTIVE_CALLED") == "1" {
					os.Exit(1)
				}
				os.Setenv("IS_ACTIVE_CALLED", "1")
				os.Exit(1)
			}
			if len(args) > 2 && args[2] == "list-units" {
				fmt.Print("frameflow-stream-V1A1\nframeflow-stream-V2A1\n")
				os.Exit(0)
			}
			fmt.Print("systemctl called")
			os.Exit(0)
		case "systemd-run":
			if os.Getenv("MOCK_SYSTEMD_RUN_FAIL") == "1" {
				os.Exit(1)
			}
			os.Exit(0)
		}
		os.Exit(1)
	}

	// Override sysutils.RunCommand
	sysutils.RunCommand = func(timeout time.Duration, name string, args ...string) (string, error) {
		cmdArgs := []string{"-test.run=TestHelperProcess", "--", name}
		cmdArgs = append(cmdArgs, args...)
		env := []string{
			"GO_WANT_HELPER_PROCESS=1",
			"MOCK_PING_FAIL=" + os.Getenv("MOCK_PING_FAIL"),
			"MOCK_SYSTEMCTL_IS_ACTIVE=" + os.Getenv("MOCK_SYSTEMCTL_IS_ACTIVE"),
			"MOCK_SYSTEMCTL_FAIL_IS_ACTIVE=" + os.Getenv("MOCK_SYSTEMCTL_FAIL_IS_ACTIVE"),
			"MOCK_SYSTEMCTL_FAIL_IS_ACTIVE_AFTER_START=" + os.Getenv("MOCK_SYSTEMCTL_FAIL_IS_ACTIVE_AFTER_START"),
			"MOCK_SYSTEMD_RUN_FAIL=" + os.Getenv("MOCK_SYSTEMD_RUN_FAIL"),
			"IS_ACTIVE_CALLED=" + os.Getenv("IS_ACTIVE_CALLED"),
		}
		return sysutils.RunCommandWithEnv(timeout, env, os.Args[0], cmdArgs...)
	}

	os.Exit(m.Run())
}

func TestHelperProcess(t *testing.T) {
	// This is just a dummy test to allow TestMain to run the helper process
}

func TestGetVideoDevice(t *testing.T) {
	tests := []struct {
		name     string
		vidID    int
		expected string
		err      bool
	}{
		{"vidID=0", 0, "", false},
		{"vidID=1", 1, "/dev/video4", false},
		{"vidID=2", 2, "/dev/video6", false},
		{"vidID=3 missing", 3, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetVideoDevice(tt.vidID)
			if (err != nil) != tt.err {
				t.Errorf("GetVideoDevice() error = %v, wantErr %v", err, tt.err)
				return
			}
			if result != tt.expected {
				t.Errorf("GetVideoDevice() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestStartStream(t *testing.T) {
	tests := []struct {
		name           string
		cameraID       string
		vidID          int
		audID          int
		strURL         string
		strMode        string
		ffmpegPath     string
		mockActive     string
		mockFailActive string
		mockRunFail    string
		expectErr      bool
	}{
		{"Already running", "V1A1", 1, 1, "srt://test", "mpegts", "/usr/bin/ffmpeg", "1", "0", "0", true},
		{"Success V1A1", "V1A1", 1, 1, "srt://test", "mpegts", "/usr/bin/ffmpeg", "0", "0", "0", false},
		{"Success V2A0", "V2A0", 2, 0, "rtsp://test", "rtsp", "/usr/bin/ffmpeg", "0", "0", "0", false},
		{"Success V0A2", "V0A2", 0, 2, "srt://test", "mpegts", "/usr/bin/ffmpeg", "0", "0", "0", false},
		{"Systemd Run Fail", "V1A1", 1, 1, "srt://test", "mpegts", "/usr/bin/ffmpeg", "0", "0", "1", true},
		{"Video device missing", "V10A1", 10, 1, "srt://test", "mpegts", "/usr/bin/ffmpeg", "0", "0", "0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("MOCK_SYSTEMCTL_IS_ACTIVE", tt.mockActive)
			os.Setenv("MOCK_SYSTEMCTL_FAIL_IS_ACTIVE", tt.mockFailActive)
			os.Setenv("MOCK_SYSTEMD_RUN_FAIL", tt.mockRunFail)
			os.Setenv("IS_ACTIVE_CALLED", "0")

			// We will just replace sysutils.RunCommand for this test to be more precise
			originalRunCommand := sysutils.RunCommand
			defer func() { sysutils.RunCommand = originalRunCommand }()

			isActiveCalledCount := 0

			sysutils.RunCommand = func(timeout time.Duration, name string, args ...string) (string, error) {
				if name == "systemctl" && len(args) > 0 {
					var isActiveCmd bool
					for _, arg := range args {
						if arg == "is-active" {
							isActiveCmd = true
							break
						}
					}
					if isActiveCmd {
						if tt.mockActive == "1" {
							return "", nil // Active
						}

						// Not active initially
						if isActiveCalledCount == 0 {
							isActiveCalledCount = 1
							return "inactive", fmt.Errorf("inactive")
						}

						// Called after start
						if tt.mockRunFail == "1" {
							return "inactive", fmt.Errorf("inactive")
						}
						return "", nil // Active after start
					}
				}
				if name == "systemd-run" && tt.mockRunFail == "1" {
					return "", fmt.Errorf("failed to run")
				}

				// fallback to originalRunCommand which delegates to the helper process properly
				// with the base environment setup in TestMain
				return originalRunCommand(timeout, name, args...)
			}

			os.Setenv("SRT_URL", tt.strURL)
			os.Setenv("RTSP_URL", tt.strURL)
			protocol := "srt"
			if tt.strMode == "rtsp" {
				protocol = "rtsp"
			}
			os.Setenv("STREAM_PROTOCOL", protocol)
			os.Setenv("FRAMEFLOW_ROLE", "CLIENT")
			defer os.Unsetenv("SRT_URL")
			defer os.Unsetenv("RTSP_URL")
			defer os.Unsetenv("STREAM_PROTOCOL")
			defer os.Unsetenv("FRAMEFLOW_ROLE")
			err := StartStream(tt.cameraID, tt.vidID, tt.audID)
			if (err != nil) != tt.expectErr {
				t.Errorf("StartStream() error = %v, wantErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestStopStream(t *testing.T) {
	err := StopStream("V1A1")
	if err != nil {
		t.Errorf("StopStream failed: %v", err)
	}
}

func TestStatusStream(t *testing.T) {
	_, err := StatusStream("V1A1")
	if err != nil {
		t.Errorf("StatusStream failed: %v", err)
	}
}

func TestStatusAllStreams(t *testing.T) {
	out, err := StatusAllStreams()
	if err != nil {
		t.Errorf("StatusAllStreams failed: %v", err)
	}
	if out == "" {
		t.Errorf("StatusAllStreams returned empty output")
	}
}

func TestPrepareStreamURL(t *testing.T) {
	tests := []struct {
		name         string
		protocol     string
		rtspURL      string
		srtURL       string
		role         string
		mockPingFail bool
		expectedURL  string
		expectedMode string
		err          bool
	}{
		{"RTSP Success", "rtsp", "rtsp://test", "", "", false, "rtsp://test", "rtsp", false},
		{"RTSP missing URL", "rtsp", "", "", "", false, "", "", true},
		{"SRT missing URL", "srt", "", "", "", false, "", "", true},
		{"SRT Success not CLIENT", "srt", "", "srt://myhost:1234", "SERVER", false, "srt://myhost:1234", "mpegts", false},
		{"SRT Success CLIENT ping fails", "srt", "", "srt://myhost:1234", "CLIENT", true, "srt://myhost:1234", "mpegts", false},
		{"SRT Success CLIENT ping succeeds", "srt", "", "srt://myhost:1234", "CLIENT", false, "srt://10.1.10.1:1234", "mpegts", false},
		{"Default fallback Success not CLIENT", "unknown", "", "srt://otherhost:5678", "SERVER", false, "srt://otherhost:5678", "mpegts", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockPingFail {
				os.Setenv("MOCK_PING_FAIL", "1")
			} else {
				os.Setenv("MOCK_PING_FAIL", "0")
			}

				// Reset global latch state before each test run
				pingMutex.Lock()
				pingFailCount = 0
				if tt.mockPingFail {
					pingFailCount = 3 // To trigger the fallback immediately
				}
				pingMutex.Unlock()

			url, mode, err := PrepareStreamURL(tt.protocol, tt.rtspURL, tt.srtURL, tt.role)
			if (err != nil) != tt.err {
				t.Errorf("PrepareStreamURL() error = %v, wantErr %v", err, tt.err)
				return
			}
			if url != tt.expectedURL {
				t.Errorf("PrepareStreamURL() url = %v, want %v", url, tt.expectedURL)
			}
			if mode != tt.expectedMode {
				t.Errorf("PrepareStreamURL() mode = %v, want %v", mode, tt.expectedMode)
			}
		})
	}
	os.Unsetenv("MOCK_PING_FAIL")
}

func TestBuildStreamURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		mode     string
		vidID    int
		audID    int
		expected string
	}{
			{"RTSP mode", "rtsp://base", "rtsp", 1, 0, "rtsp://base/_1"},
			{"RTSP mode vidID 0", "rtsp://base", "rtsp", 0, 1, "rtsp://base/_A1"},
			{"MPEGTS plain", "srt://base", "mpegts", 2, 0, "srt://base/_2"},
		{"MPEGTS publish", "srt://base?streamid=publish:test", "mpegts", 1, 0, "srt://base?streamid=publish:test_1"},
		{"MPEGTS publish auth", "srt://base?streamid=publish:test:user:pass", "mpegts", 1, 0, "srt://base?streamid=publish:test_1:user:pass"},
		{"MPEGTS publish auth vidID 0", "srt://base?streamid=publish:test:user:pass", "mpegts", 0, 2, "srt://base?streamid=publish:test_A2:user:pass"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildStreamURL(tt.baseURL, tt.mode, tt.vidID, tt.audID)
			if result != tt.expected {
				t.Errorf("BuildStreamURL() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseCameraID(t *testing.T) {
	tests := []struct {
		camID       string
		expectedVid int
		expectedAud int
		err         bool
	}{
		{"V1A1", 1, 1, false},
		{"V2A0", 2, 0, false},
		{"V0A1", 0, 1, false},
		{"V0A0", 0, 0, true},
		{"V1", 0, 0, true},
		{"A1", 0, 0, true},
		{"V1Aabc", 0, 0, true},
		{"V-1A1", 0, 0, true},
		{"", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.camID, func(t *testing.T) {
			vidID, audID, err := ParseCameraID(tt.camID)
			if (err != nil) != tt.err {
				t.Errorf("ParseCameraID() error = %v, wantErr %v", err, tt.err)
				return
			}
			if vidID != tt.expectedVid {
				t.Errorf("ParseCameraID() vidID = %v, want %v", vidID, tt.expectedVid)
			}
			if audID != tt.expectedAud {
				t.Errorf("ParseCameraID() audID = %v, want %v", audID, tt.expectedAud)
			}
		})
	}
}

func TestGetAudioDevice(t *testing.T) {
	tests := []struct {
		name     string
		audID    int
		expected string
		err      bool
	}{
		{"audID=0", 0, "", false},
		{"audID=1", 1, "0", false},
		{"audID=2", 2, "1", false},
		{"audID=3", 3, "3", false},
		{"audID=4 missing", 4, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetAudioDevice(tt.audID)
			if (err != nil) != tt.err {
				t.Errorf("GetAudioDevice() error = %v, wantErr %v", err, tt.err)
				return
			}
			if result != tt.expected {
				t.Errorf("GetAudioDevice() = %v, want %v", result, tt.expected)
			}
		})
	}
}
