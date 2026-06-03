package mediamtx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstallMediaMTX(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("MEDIAMTX_DIR", tempDir)
	defer os.Unsetenv("MEDIAMTX_DIR")

	// Store original mock setup
	originalRunCommandWithEnv := runCommandWithEnv
	defer func() { runCommandWithEnv = originalRunCommandWithEnv }()

	t.Run("Valid Installation", func(t *testing.T) {
		runCommandWithEnv = func(timeout time.Duration, env []string, name string, args ...string) (string, error) {
			cmdStr := name + " " + strings.Join(args, " ")
			if strings.Contains(cmdStr, "wget") && strings.Contains(cmdStr, "releases/latest") {
				return `{"assets": [{"name": "mediamtx_linux_arm64.tar.gz", "browser_download_url": "http://example.com/mediamtx.tar.gz"}, {"name": "checksums.txt", "browser_download_url": "http://example.com/checksums.txt"}]}`, nil
			}
			if strings.Contains(cmdStr, "wget") && strings.Contains(cmdStr, "mediamtx.tar.gz") {
				os.WriteFile(filepath.Join(tempDir, "mediamtx.tar.gz"), []byte("dummy content"), 0644)
				return "", nil
			}
			if strings.Contains(cmdStr, "wget") && strings.Contains(cmdStr, "checksums.txt") {
				// We need actual checksum to pass
				hash := "b5a2c96250612366ea272ffac6d9744aaf4b45aacd96a735a1e4ebb0c4ed7bc6" // dummy content hash
				os.WriteFile(filepath.Join(tempDir, "checksums.txt"), []byte(hash+"  mediamtx.tar.gz"), 0644)
				return "", nil
			}
			if strings.Contains(cmdStr, "jq") && !strings.Contains(cmdStr, "browser_download_url") {
				return "", nil
			}
			if strings.Contains(cmdStr, "browser_download_url") {
				if strings.Contains(cmdStr, "linux_arm64") || strings.Contains(cmdStr, "linux_amd64") {
					return "http://example.com/mediamtx.tar.gz", nil
				}
				if strings.Contains(cmdStr, "checksums") {
					return "http://example.com/checksums.txt", nil
				}
			}
			if strings.Contains(cmdStr, "sha256sum") {
				return "b5a2c96250612366ea272ffac6d9744aaf4b45aacd96a735a1e4ebb0c4ed7bc6", nil
			}
			if strings.Contains(cmdStr, "grep") && strings.Contains(cmdStr, "awk") {
				return "b5a2c96250612366ea272ffac6d9744aaf4b45aacd96a735a1e4ebb0c4ed7bc6", nil
			}
			if strings.Contains(cmdStr, "tar -zxf") {
				return "", nil
			}
			if strings.Contains(cmdStr, "chown") || strings.Contains(cmdStr, "chmod") {
				return "", nil
			}
			if strings.Contains(cmdStr, "ls -ld") {
				return "root root root", nil
			}
			return "", nil
		}

		err := Install()
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
	})

	t.Run("Empty Response", func(t *testing.T) {
		runCommandWithEnv = func(timeout time.Duration, env []string, name string, args ...string) (string, error) {
			cmdStr := name + " " + strings.Join(args, " ")
			if strings.Contains(cmdStr, "wget") && strings.Contains(cmdStr, "releases/latest") {
				return "", nil
			}
			return "", nil
		}

		err := Install()
		if err == nil {
			t.Errorf("Expected error for empty response")
		} else if !strings.Contains(err.Error(), "release metadata is empty") {
			t.Errorf("Expected specific empty response error, got %v", err)
		}
	})

	t.Run("Wget Failure", func(t *testing.T) {
		runCommandWithEnv = func(timeout time.Duration, env []string, name string, args ...string) (string, error) {
			cmdStr := name + " " + strings.Join(args, " ")
			if strings.Contains(cmdStr, "wget") && strings.Contains(cmdStr, "releases/latest") {
				return "", fmt.Errorf("wget failed")
			}
			return "", nil
		}

		err := Install()
		if err == nil {
			t.Errorf("Expected error for wget failure")
		} else if !strings.Contains(err.Error(), "failed to fetch metadata") {
			t.Errorf("Expected specific fetch error, got %v", err)
		}
	})

	t.Run("Invalid JSON Response", func(t *testing.T) {
		runCommandWithEnv = func(timeout time.Duration, env []string, name string, args ...string) (string, error) {
			cmdStr := name + " " + strings.Join(args, " ")
			if strings.Contains(cmdStr, "wget") && strings.Contains(cmdStr, "releases/latest") {
				return "<html><body>Captive Portal</body></html>", nil
			}
			if strings.Contains(cmdStr, "jq -e") {
				return "", fmt.Errorf("jq parse error")
			}
			return "", nil
		}

		err := Install()
		if err == nil {
			t.Errorf("Expected error for invalid JSON")
		} else if !strings.Contains(err.Error(), "metadata is not valid JSON") {
			t.Errorf("Expected specific json error, got %v", err)
		}
	})
}

func TestStartMediaMTX(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("MEDIAMTX_DIR", tempDir)
	defer os.Unsetenv("MEDIAMTX_DIR")

	vlxTempDir := t.TempDir()
	os.Setenv("VLXsuite_DIR", vlxTempDir)
	defer os.Unsetenv("VLXsuite_DIR")

	os.MkdirAll(filepath.Join(vlxTempDir, "config"), 0755)
	os.MkdirAll(filepath.Join(vlxTempDir, "etc"), 0755)
	templatePath := filepath.Join(vlxTempDir, "etc", "mediamtx.settings")
	os.WriteFile(templatePath, []byte("runOnReady: ffmpeg_placeholder\n"), 0644)
	os.WriteFile(filepath.Join(vlxTempDir, "etc", "frameflow.settings"), []byte("DUMMY=1\n"), 0644)

	os.Setenv("SRT_URL", "srt://127.0.0.1:8889?streamid=publish:cam")
	defer os.Unsetenv("SRT_URL")

	originalRunCommandWithEnv := runCommandWithEnv
	defer func() { runCommandWithEnv = originalRunCommandWithEnv }()

	t.Run("Happy Path", func(t *testing.T) {
		runCommandWithEnv = func(timeout time.Duration, env []string, name string, args ...string) (string, error) {
			cmdStr := name + " " + strings.Join(args, " ")
			if strings.Contains(cmdStr, "systemctl") && strings.Contains(cmdStr, "is-active") {
				return "", fmt.Errorf("inactive") // Not active initially
			}
			if strings.Contains(cmdStr, "sed -i") {
				return "", nil
			}
			if strings.Contains(cmdStr, "systemd-run") {
				return "running", nil
			}
			return "", nil
		}

		err := Start()
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
	})

	t.Run("Service Already Running", func(t *testing.T) {
		runCommandWithEnv = func(timeout time.Duration, env []string, name string, args ...string) (string, error) {
			cmdStr := name + " " + strings.Join(args, " ")
			if strings.Contains(cmdStr, "systemctl") && strings.Contains(cmdStr, "is-active") {
				return "", nil // Return success (active)
			}
			if strings.Contains(cmdStr, "status") {
				return "status info", nil
			}
			return "", nil
		}

		err := Start()
		if err == nil {
			t.Errorf("Expected error for already running")
		} else if err.Error() != "service already running" {
			t.Errorf("Expected specific error, got %v", err)
		}
	})

	t.Run("Missing Template", func(t *testing.T) {
		os.Remove(templatePath)
		defer os.WriteFile(templatePath, []byte("runOnReady: ffmpeg_placeholder\n"), 0644)

		runCommandWithEnv = func(timeout time.Duration, env []string, name string, args ...string) (string, error) {
			cmdStr := name + " " + strings.Join(args, " ")
			if strings.Contains(cmdStr, "systemctl") && strings.Contains(cmdStr, "is-active") {
				return "", fmt.Errorf("inactive")
			}
			return "", nil
		}

		err := Start()
		if err == nil {
			t.Errorf("Expected error for missing template")
		} else if err.Error() != "template file not found" {
			t.Errorf("Expected specific error, got %v", err)
		}
	})

	t.Run("SRT_URL Not Set", func(t *testing.T) {
		os.Unsetenv("SRT_URL")

		runCommandWithEnv = func(timeout time.Duration, env []string, name string, args ...string) (string, error) {
			cmdStr := name + " " + strings.Join(args, " ")
			if strings.Contains(cmdStr, "systemctl") && strings.Contains(cmdStr, "is-active") {
				return "", fmt.Errorf("inactive")
			}
			return "", nil
		}

		err := Start()
		if err == nil {
			t.Errorf("Expected error for SRT_URL not set")
		} else if err.Error() != "SRT_URL not set" {
			t.Errorf("Expected specific error, got %v", err)
		}

		// Restore for other tests if any
		os.Setenv("SRT_URL", "srt://127.0.0.1:8889?streamid=publish:cam")
	})

	t.Run("Client Role Delayed Ping", func(t *testing.T) {
		os.Setenv("FRAMEFLOW_ROLE", "CLIENT")
		defer os.Unsetenv("FRAMEFLOW_ROLE")

		pingCount := 0
		var sedCmdExecuted string

		runCommandWithEnv = func(timeout time.Duration, env []string, name string, args ...string) (string, error) {
			cmdStr := name + " " + strings.Join(args, " ")
			if strings.Contains(cmdStr, "systemctl") && strings.Contains(cmdStr, "is-active") {
				if strings.Contains(cmdStr, "frameflow-mptcp-proxy.service") || strings.Contains(cmdStr, "frameflow-mlvpn.service") {
					return "active", nil
				}
				return "", fmt.Errorf("inactive")
			}
			if strings.Contains(cmdStr, "ping") {
				pingCount++
				if pingCount < 3 {
					return "", fmt.Errorf("ping timeout")
				}
				return "pong", nil // succeeds on 3rd try
			}
			if strings.Contains(cmdStr, "sed -i") {
				sedCmdExecuted = cmdStr
				return "", nil
			}
			if strings.Contains(cmdStr, "systemd-run") {
				return "running", nil
			}
			return "", nil
		}

		err := Start()
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
		if pingCount != 3 {
			t.Errorf("Expected 3 pings, got %d", pingCount)
		}
		if !strings.Contains(sedCmdExecuted, "srt://10.1.10.1:8889?streamid=publish:cam_mtx") {
			t.Errorf("Expected sed command to contain updated SRT URL, got: %s", sedCmdExecuted)
		}
	})

	t.Run("Start Failure", func(t *testing.T) {
		runCommandWithEnv = func(timeout time.Duration, env []string, name string, args ...string) (string, error) {
			cmdStr := name + " " + strings.Join(args, " ")
			if strings.Contains(cmdStr, "systemctl") && strings.Contains(cmdStr, "is-active") {
				// In Start() it checks if active initially, then again after starting
				if true {
					return "", fmt.Errorf("inactive") // First check
				}
				return "", fmt.Errorf("failed") // Second check
			}
			if strings.Contains(cmdStr, "sed -i") {
				return "", nil
			}
			if strings.Contains(cmdStr, "systemd-run") {
				return "", fmt.Errorf("systemd-run error")
			}
			if strings.Contains(cmdStr, "journalctl") {
				return "journal logs", nil
			}
			return "", nil
		}

		err := Start()
		if err == nil {
			t.Errorf("Expected error for start failure")
		}
	})
}

var sysutilsLoggerMsg = ""

func init() {
	// Dummy to satisfy any logging check if needed

}

func TestStopMediaMTX(t *testing.T) {
	vlxTempDir := t.TempDir()
	os.Setenv("VLXsuite_DIR", vlxTempDir)
	defer os.Unsetenv("VLXsuite_DIR")
	os.MkdirAll(filepath.Join(vlxTempDir, "etc"), 0755)
	os.WriteFile(filepath.Join(vlxTempDir, "etc", "frameflow.settings"), []byte("DUMMY=1\n"), 0644)

	originalRunCommandWithEnv := runCommandWithEnv
	defer func() { runCommandWithEnv = originalRunCommandWithEnv }()

	runCommandWithEnv = func(timeout time.Duration, env []string, name string, args ...string) (string, error) {
		cmdStr := name + " " + strings.Join(args, " ")
		if strings.Contains(cmdStr, "systemctl") && strings.Contains(cmdStr, "stop") && strings.Contains(cmdStr, "frameflow-mediamtx") {
			return "stopped", nil
		}
		return "", fmt.Errorf("unexpected command: %s", cmdStr)
	}

	err := Stop()
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

func TestStatusMediaMTX(t *testing.T) {
	vlxTempDir := t.TempDir()
	os.Setenv("VLXsuite_DIR", vlxTempDir)
	defer os.Unsetenv("VLXsuite_DIR")
	os.MkdirAll(filepath.Join(vlxTempDir, "etc"), 0755)
	os.WriteFile(filepath.Join(vlxTempDir, "etc", "frameflow.settings"), []byte("DUMMY=1\n"), 0644)

	originalRunCommandWithEnv := runCommandWithEnv
	defer func() { runCommandWithEnv = originalRunCommandWithEnv }()

	runCommandWithEnv = func(timeout time.Duration, env []string, name string, args ...string) (string, error) {
		cmdStr := name + " " + strings.Join(args, " ")
		if strings.Contains(cmdStr, "systemctl") && strings.Contains(cmdStr, "is-active") && strings.Contains(cmdStr, "frameflow-mediamtx") {
			// Simulate active so fallback cleanup is not triggered
			return "", nil
		}
		if strings.Contains(cmdStr, "systemctl") && strings.Contains(cmdStr, "status") && strings.Contains(cmdStr, "frameflow-mediamtx") {
			return "active (running)", nil
		}
		return "", fmt.Errorf("unexpected command: %s", cmdStr)
	}

	err := Status()
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

// Global logger msg to track state in mock
func setSysutilsGlobalLoggerMsg(msg string) {

}
