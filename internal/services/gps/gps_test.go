package gps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		cmd := os.Args[3]
		switch cmd {
		case "bash":
			if os.Getenv("MOCK_DMESG") == "1" {
				fmt.Print(os.Getenv("DMESG_OUTPUT"))
				if os.Getenv("DMESG_FAIL") == "1" {
					os.Exit(1)
				}
				os.Exit(0)
			}
			if os.Getenv("MOCK_LS") == "1" {
				fmt.Print(os.Getenv("LS_OUTPUT"))
				os.Exit(0)
			}
		case "systemctl":
			args := os.Args[3:]
			if len(args) > 2 && args[2] == "is-active" {
				if os.Getenv("MOCK_IS_ACTIVE") == "1" {
					os.Exit(0) // Active
				}
				os.Exit(1) // Not active
			}
			if len(args) > 2 && args[2] == "stop" {
				os.Exit(0)
			}
			if len(args) > 2 && args[2] == "status" {
				fmt.Print("Status Output")
				os.Exit(0)
			}
		case "which":
			if os.Getenv("MOCK_WHICH_FAIL") == "1" {
				os.Exit(1)
			}
			fmt.Print("/usr/bin/gpsd")
			os.Exit(0)
		case "systemd-run":
			if os.Getenv("MOCK_SYSTEMD_RUN_FAIL") == "1" {
				os.Exit(1)
			}
			os.Exit(0)
		}
		os.Exit(1)
	}

	sysutils.RunCommand = func(timeout time.Duration, name string, args ...string) (string, error) {
		cmdArgs := []string{"-test.run=TestHelperProcess", "--", name}
		cmdArgs = append(cmdArgs, args...)
		env := []string{
			"GO_WANT_HELPER_PROCESS=1",
			"MOCK_DMESG=" + os.Getenv("MOCK_DMESG"),
			"DMESG_OUTPUT=" + os.Getenv("DMESG_OUTPUT"),
			"DMESG_FAIL=" + os.Getenv("DMESG_FAIL"),
			"MOCK_LS=" + os.Getenv("MOCK_LS"),
			"LS_OUTPUT=" + os.Getenv("LS_OUTPUT"),
			"MOCK_IS_ACTIVE=" + os.Getenv("MOCK_IS_ACTIVE"),
			"MOCK_WHICH_FAIL=" + os.Getenv("MOCK_WHICH_FAIL"),
			"MOCK_SYSTEMD_RUN_FAIL=" + os.Getenv("MOCK_SYSTEMD_RUN_FAIL"),
		}
		return sysutils.RunCommandWithEnv(timeout, env, os.Args[0], cmdArgs...)
	}

	os.Exit(m.Run())
}

func TestHelperProcess(t *testing.T) {
	// Dummy test
}

func TestGetGPSDevice(t *testing.T) {
	tempDir := t.TempDir()
	originalDevDir := DevDir
	DevDir = tempDir
	defer func() { DevDir = originalDevDir }()

	// Create dummy device files
	usb0 := tempDir + "/ttyUSB0"
	acm0 := tempDir + "/ttyACM0"
	usb2 := tempDir + "/ttyUSB2"

	os.WriteFile(usb0, []byte(""), 0644)
	os.WriteFile(acm0, []byte(""), 0644)
	os.WriteFile(usb2, []byte(""), 0644)

	now := time.Now()
	// ACM0 is the newest
	os.Chtimes(usb0, now.Add(-10*time.Minute), now.Add(-10*time.Minute))
	os.Chtimes(usb2, now.Add(-5*time.Minute), now.Add(-5*time.Minute))
	os.Chtimes(acm0, now, now)

	originalRunCommand := sysutils.RunCommand
	defer func() { sysutils.RunCommand = originalRunCommand }()

	sysutils.RunCommand = func(timeout time.Duration, name string, args ...string) (string, error) {
		if name == "udevadm" {
			// Mock udevadm behavior
			if len(args) > 5 {
				dev := args[5]
				if dev == usb0 {
					return "ID_MODEL=some generic serial", nil
				}
				if dev == usb2 {
					return "ID_MODEL=Sierra Wireless LTE Modem", nil
				}
				if dev == acm0 {
					return "ID_MODEL=u-blox GNSS receiver", nil
				}
			}
		}
		return "", nil
	}

	dev, err := GetGPSDevice()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should pick acm0 because it is the newest non-modem device.
	// USB2 is newer than USB0, but it is an LTE modem, so it should be skipped.
	if dev != "ttyACM0" {
		t.Errorf("expected %q, got %q", "ttyACM0", dev)
	}

	// Test no devices
	DevDir = t.TempDir()
	dev, err = GetGPSDevice()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dev != "" {
		t.Errorf("expected empty string when no devices exist, got %q", dev)
	}
}

func TestStartGPSD(t *testing.T) {
	tempDir := t.TempDir()
	originalDevDir := DevDir
	DevDir = tempDir
	defer func() { DevDir = originalDevDir }()

	tests := []struct {
		name          string
		mockActive    string
		mockWhichFail string
		mockRunFail   string
		setupDev      func()
		expectErr     bool
	}{
		{"Already running", "1", "0", "0", func() { os.WriteFile(tempDir+"/ttyACM0", []byte(""), 0644) }, false},
		{"Success", "0", "0", "0", func() { os.WriteFile(tempDir+"/ttyACM0", []byte(""), 0644) }, false},
		{"No device", "0", "0", "0", func() { os.Remove(tempDir + "/ttyACM0") }, true},
		{"Which fails", "0", "1", "0", func() { os.WriteFile(tempDir+"/ttyACM0", []byte(""), 0644) }, true},
		{"Systemd Run fails", "0", "0", "1", func() { os.WriteFile(tempDir+"/ttyACM0", []byte(""), 0644) }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("MOCK_IS_ACTIVE", tt.mockActive)
			os.Setenv("MOCK_WHICH_FAIL", tt.mockWhichFail)
			os.Setenv("MOCK_SYSTEMD_RUN_FAIL", tt.mockRunFail)

			tt.setupDev()

			originalRunCommand := sysutils.RunCommand
			defer func() { sysutils.RunCommand = originalRunCommand }()

			sysutils.RunCommand = func(timeout time.Duration, name string, args ...string) (string, error) {
				if name == "udevadm" {
					return "", nil
				}
				return originalRunCommand(timeout, name, args...)
			}

			err := StartGPSD("1198")
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error: %v, got: %v", tt.expectErr, err)
			}
		})
	}
}

func TestStopGPSD(t *testing.T) {
	err := StopGPSD()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStatusGPSD(t *testing.T) {
	status, err := StatusGPSD()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(status) == 0 {
		t.Errorf("expected status output, got empty")
	}
}

func TestRunSender(t *testing.T) {
	// Start Mock API Server
	var receivedPayload Payload
	var authToken string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authToken = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	// Start Mock GPSD TCP Server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen on tcp: %v", err)
	}
	defer listener.Close()

	_, port, _ := net.SplitHostPort(listener.Addr().String())

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read the ?WATCH command
		buf := make([]byte, 1024)
		conn.Read(buf)

		// Send dummy TPV json
		dummyTPV := `{"class":"TPV","lat":37.7749,"lon":-122.4194,"altMSL":10.5,"epx":1.2,"speed":5.5}
`
		conn.Write([]byte(dummyTPV))
	}()

	ctx, cancel := context.WithCancel(context.Background())

	// Run sender in a goroutine
	done := make(chan error)
	go func() {
		err := RunSender(ctx, port, apiServer.URL, "secret-token")
		done <- err
	}()

	// Wait a moment for data to be sent
	time.Sleep(500 * time.Millisecond)
	cancel() // stop the sender
	<-done

	if authToken != "Bearer secret-token" {
		t.Errorf("expected token 'Bearer secret-token', got '%s'", authToken)
	}

	if receivedPayload.Lat != 37.7749 || receivedPayload.Lon != -122.4194 {
		t.Errorf("payload not received correctly, got: %+v", receivedPayload)
	}
}
