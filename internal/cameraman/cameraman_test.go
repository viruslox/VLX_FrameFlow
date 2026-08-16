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
		case "ip":
			if os.Getenv("MOCK_MLVPN_UP") == "1" && os.Args[6] == "mlvpn0" {
				fmt.Print("state UP")
				os.Exit(0)
			}
			if os.Getenv("MOCK_TUN_UP") == "1" && os.Args[6] == "tun0" {
				fmt.Print("state UP")
				os.Exit(0)
			}
			os.Exit(1)
		case "curl":
			if os.Getenv("MOCK_CURL_FAIL") == "1" {
				os.Exit(1)
			}
			if os.Getenv("MOCK_CURL_NOT_FOUND") == "1" {
				fmt.Print("not found")
				os.Exit(0)
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
			"MOCK_MLVPN_UP=" + os.Getenv("MOCK_MLVPN_UP"),
			"MOCK_TUN_UP=" + os.Getenv("MOCK_TUN_UP"),
			"MOCK_CURL_FAIL=" + os.Getenv("MOCK_CURL_FAIL"),
			"MOCK_CURL_NOT_FOUND=" + os.Getenv("MOCK_CURL_NOT_FOUND"),
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
		{"vidID=0", 0, "", true},
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
	err := StartStream("V10", "")
	if err == nil {
		t.Errorf("StartStream should fail for missing device")
	}
}

func TestStopStream(t *testing.T) {
	// Without a mock HTTP server, StopStream to 127.0.0.1:9997 will fail with connection refused.
	err := StopStream("01")
	if err == nil {
		t.Errorf("StopStream should fail without running mediamtx")
	}
}

func TestStatusStream(t *testing.T) {
	_, err := StatusStream("01")
	if err == nil {
		t.Errorf("StatusStream should fail when DB is not properly initialized for testing, got nil")
	}
}

func TestStatusAllStreams(t *testing.T) {
	_, err := StatusAllStreams()
	if err != nil {
		t.Errorf("StatusAllStreams failed: %v", err)
	}
}

func TestPrepareStreamURL(t *testing.T) {
	tests := []struct {
		name         string
		srtURL       string
		expectedSRT  string
	}{
		{"Test URL", "srt://test", "srt://test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Without a real ping mock, we just test it runs without panicking.
			// Expected return will likely be the unmodified srtURL since the real ping will probably fail in test env.
			srt := PrepareStreamURL(tt.srtURL)
			if srt != tt.expectedSRT && srt != "srt://10.1.10.1" {
				t.Errorf("PrepareStreamURL() srt = %v, want %v or srt://10.1.10.1", srt, tt.expectedSRT)
			}
		})
	}
}

func TestParseCameraID(t *testing.T) {
	tests := []struct {
		camID        string
		expectedVID  int
		expectedAID  int
		hasVideo     bool
		hasAudio     bool
		expectExpAud bool
		err          bool
	}{
		{"V1", 1, 0, true, false, false, false},        // video + auto audio
		{"V1A2", 1, 2, true, true, true, false},        // explicit audio
		{"V1A0", 1, 0, true, false, true, false},       // video + silent
		{"V0A1", 0, 1, false, true, true, false},       // audio-only + dark-grey video
		{"V0", 0, 0, false, false, false, true},        // invalid: V0 alone
		{"V0A0", 0, 0, false, false, false, true},      // invalid: nothing real
		{"A0", 0, 0, false, false, false, true},        // invalid: no V prefix
		{"A2", 0, 0, false, false, false, true},        // invalid: no V prefix
		{"abc", 0, 0, false, false, false, true},
		{"V", 0, 0, false, false, false, true},
		{"", 0, 0, false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.camID, func(t *testing.T) {
			spec, err := ParseCameraID(tt.camID)
			if (err != nil) != tt.err {
				t.Errorf("ParseCameraID() error = %v, wantErr %v", err, tt.err)
				return
			}
			if tt.err {
				return
			}
			if spec.VideoID != tt.expectedVID {
				t.Errorf("ParseCameraID() VideoID = %v, want %v", spec.VideoID, tt.expectedVID)
			}
			if spec.AudioID != tt.expectedAID {
				t.Errorf("ParseCameraID() AudioID = %v, want %v", spec.AudioID, tt.expectedAID)
			}
			if spec.HasVideo != tt.hasVideo {
				t.Errorf("ParseCameraID() HasVideo = %v, want %v", spec.HasVideo, tt.hasVideo)
			}
			if spec.HasAudio != tt.hasAudio {
				t.Errorf("ParseCameraID() HasAudio = %v, want %v", spec.HasAudio, tt.hasAudio)
			}
			if spec.ExplicitAudio != tt.expectExpAud {
				t.Errorf("ParseCameraID() ExplicitAudio = %v, want %v", spec.ExplicitAudio, tt.expectExpAud)
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
