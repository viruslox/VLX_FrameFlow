package mediamtx

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

var runCommand = sysutils.RunCommand
var runCommandWithEnv = sysutils.RunCommandWithEnv

func loadEnv() {
	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}
	settingsFile := filepath.Join(vlxSuiteDir, "etc", "frameflow.settings")

	content, err := os.ReadFile(settingsFile)
	if err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"'`)
				if os.Getenv(key) == "" {
					os.Setenv(key, val)
				}
			}
		}
	}
}

func getUserEnv() []string {
	uid := os.Getuid()
	if uid == 0 {
		sudoUser := os.Getenv("SUDO_USER")
		if sudoUser != "" {
			out, err := runCommandWithEnv(10*time.Second, nil, "id", "-u", sudoUser)
			if err == nil {
				uidStr := strings.TrimSpace(out)
				return []string{
					fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%s", uidStr),
					fmt.Sprintf("DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/%s/bus", uidStr),
				}
			}
		}
	}
	return []string{
		fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%d", uid),
		fmt.Sprintf("DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/%d/bus", uid),
	}
}

// Install checks if MediaMTX is installed, and if not, downloads and installs it.
func Install() error {
	loadEnv()
	sysutils.Info("Checking MediaMTX...")

	mediaMtxDir := os.Getenv("MEDIAMTX_DIR")
	if mediaMtxDir == "" {
		mediaMtxDir = "/opt/mediamtx"
	}

	err := os.MkdirAll(mediaMtxDir, 0755)
	if err != nil {
		sysutils.Error("Failed to create MediaMTX directory: %v", err)
		return err
	}

	mediaMtxPath := filepath.Join(mediaMtxDir, "mediamtx")
	if _, err := os.Stat(mediaMtxPath); err == nil {
		_, cmdErr := runCommandWithEnv(10*time.Minute, nil, mediaMtxPath, "--upgrade")
		if cmdErr != nil {
			sysutils.Error("Failed to upgrade MediaMTX: %v", cmdErr)
			return cmdErr
		}

		if os.Getenv("FRAMEFLOW_ROLE") == "SERVER" {
			user := getInstalledUser()
			runCommand(10*time.Second, "chown", "-R", fmt.Sprintf("%s:%s", user, user), mediaMtxDir)
		}
		runCommandWithEnv(10*time.Second, nil, "chmod", "700", mediaMtxPath)

		sysutils.Success("MediaMTX ready.")
		return nil
	}

	// Fetch release metadata
	releaseJSON, err := runCommandWithEnv(1*time.Minute, nil, "wget", "--https-only", "--secure-protocol=TLSv1_2", "-qO-", "https://api.github.com/repos/bluenviron/mediamtx/releases/latest")
	if err != nil {
		sysutils.Error("Failed to fetch MediaMTX release metadata.")
		return fmt.Errorf("failed to fetch metadata: %w", err)
	}

	if strings.TrimSpace(releaseJSON) == "" {
		sysutils.Error("MediaMTX release metadata is empty.")
		return fmt.Errorf("release metadata is empty")
	}

	_, err = runCommandWithEnv(10*time.Second, nil, "command", "-v", "jq")
	if err != nil {
		sysutils.Error("jq is required but not installed.")
		return fmt.Errorf("jq not installed")
	}

	// jq expects unquoted JSON input or valid JSON format, we pass through echo
	_, err = runCommandWithEnv(10*time.Second, nil, "bash", "-c", fmt.Sprintf("echo '%s' | jq -e . >/dev/null 2>&1", strings.ReplaceAll(releaseJSON, "'", "'\\''")))
	if err != nil {
		sysutils.Error("MediaMTX release metadata is not valid JSON.")
		return fmt.Errorf("metadata is not valid JSON")
	}

	arch := runtime.GOARCH
	archSuffix := "linux_" + arch + ".tar.gz"

	urlCmd := fmt.Sprintf("echo '%s' | jq -r --arg sfx '%s' '[.assets[] | select(.name | endswith($sfx))] | first | .browser_download_url // empty'", strings.ReplaceAll(releaseJSON, "'", "'\\''"), archSuffix)
	urlOutput, err := runCommandWithEnv(10*time.Second, nil, "bash", "-c", urlCmd)
	if err != nil {
		sysutils.Error("Failed to parse download URL.")
		return err
	}
	url := strings.TrimSpace(urlOutput)

	checksumUrlCmd := fmt.Sprintf("echo '%s' | jq -r '[.assets[] | select(.name | test(\"checksums.txt|checksums.sha256\"))] | first | .browser_download_url // empty'", strings.ReplaceAll(releaseJSON, "'", "'\\''"))
	checksumUrlOutput, err := runCommandWithEnv(10*time.Second, nil, "bash", "-c", checksumUrlCmd)
	if err != nil {
		sysutils.Error("Failed to parse checksum URL.")
		return err
	}
	checksumUrl := strings.TrimSpace(checksumUrlOutput)

	if url != "" && checksumUrl != "" {
		tarPath := filepath.Join(mediaMtxDir, "mediamtx.tar.gz")
		checksumsPath := filepath.Join(mediaMtxDir, "checksums.txt")

		_, err = runCommandWithEnv(5*time.Minute, nil, "wget", "--https-only", "--secure-protocol=TLSv1_2", "-q", url, "-O", tarPath)
		if err != nil {
			return err
		}
		_, err = runCommandWithEnv(1*time.Minute, nil, "wget", "--https-only", "--secure-protocol=TLSv1_2", "-q", checksumUrl, "-O", checksumsPath)
		if err != nil {
			return err
		}

		tarFilename := filepath.Base(url)

		expectedChecksumOutput, err := runCommandWithEnv(10*time.Second, nil, "bash", "-c", fmt.Sprintf("grep '%s' '%s' | awk '{print $1}'", tarFilename, checksumsPath))
		if err != nil {
			return err
		}
		expectedChecksum := strings.TrimSpace(expectedChecksumOutput)

		actualChecksumOutput, err := runCommandWithEnv(30*time.Second, nil, "bash", "-c", fmt.Sprintf("sha256sum '%s' | awk '{print $1}'", tarPath))
		if err != nil {
			return err
		}
		actualChecksum := strings.TrimSpace(actualChecksumOutput)

		if expectedChecksum != "" && expectedChecksum == actualChecksum {
			sysutils.Info("Checksum verified (%s).", actualChecksum)

			_, err = runCommandWithEnv(1*time.Minute, nil, "bash", "-c", fmt.Sprintf("cd '%s' && tar -zxf mediamtx.tar.gz && rm mediamtx.tar.gz checksums.txt", mediaMtxDir))
			if err != nil {
				return err
			}

			if os.Getenv("FRAMEFLOW_ROLE") == "SERVER" {
				user := getInstalledUser()
				runCommand(10*time.Second, "chown", "-R", fmt.Sprintf("%s:%s", user, user), mediaMtxDir)
			}

			_, err = runCommandWithEnv(10*time.Second, nil, "chmod", "700", mediaMtxPath)
			if err != nil {
				return err
			}
		} else {
			sysutils.Error("Security: Checksum verification failed! Expected: %s, Actual: %s", expectedChecksum, actualChecksum)
			os.Remove(tarPath)
			os.Remove(checksumsPath)
			return fmt.Errorf("checksum verification failed")
		}
	} else {
		sysutils.Error("MediaMTX download failed or checksum not found.")
		return fmt.Errorf("download failed or checksum not found")
	}

	err = GenerateService()
	if err != nil {
		return err
	}

	sysutils.Info("Enabling and starting MediaMTX service...")

	isServer := os.Getenv("FRAMEFLOW_ROLE") == "SERVER"
	var userEnv []string
	var systemctlArgs []string

	if isServer {
		userEnv = nil
		systemctlArgs = []string{"systemctl"}
	} else {
		userEnv = getUserEnv()
		systemctlArgs = []string{"systemctl", "--user"}
	}

	_, err = runCommandWithEnv(10*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "daemon-reload")...)
	if err != nil {
		sysutils.Error("Failed to reload systemd daemon: %v", err)
	}

	_, err = runCommandWithEnv(10*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "enable", "frameflow-mediamtx.service")...)
	if err != nil {
		sysutils.Error("Failed to enable MediaMTX service: %v", err)
	}

	sysutils.Success("MediaMTX ready.")
	return nil
}

// Uninstall removes the static MediaMTX service
func Uninstall() error {
	sysutils.Info("Uninstalling MediaMTX service...")
	isServer := os.Getenv("FRAMEFLOW_ROLE") == "SERVER"

	var userEnv []string
	var systemctlArgs []string

	if isServer {
		userEnv = nil
		systemctlArgs = []string{"systemctl"}
	} else {
		userEnv = getUserEnv()
		systemctlArgs = []string{"systemctl", "--user"}
	}

	_, err := runCommandWithEnv(30*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "stop", "frameflow-mediamtx.service")...)
	if err != nil {
		sysutils.Info("Note: Failed to stop service (it may not be running).")
	}

	_, err = runCommandWithEnv(10*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "disable", "frameflow-mediamtx.service")...)
	if err != nil {
		sysutils.Info("Note: Failed to disable service.")
	}

	serviceDir := os.Getenv("TEST_SERVICE_DIR")
	if serviceDir == "" {
		if isServer {
			serviceDir = "/etc/systemd/system"
		} else {
			home, errHome := os.UserHomeDir()
			if errHome != nil {
				sysutils.Error("Failed to get user home directory: %v", errHome)
				return errHome
			}
			serviceDir = filepath.Join(home, ".config", "systemd", "user")
		}
	}
	servicePath := filepath.Join(serviceDir, "frameflow-mediamtx.service")
	err = os.Remove(servicePath)
	if err != nil && !os.IsNotExist(err) {
		sysutils.Error("Failed to delete service file: %v", err)
		return err
	}

	_, err = runCommandWithEnv(10*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "daemon-reload")...)
	if err != nil {
		sysutils.Error("Failed to reload systemd daemon: %v", err)
	}

	sysutils.Success("MediaMTX service uninstalled successfully.")
	return nil
}

// Start starts the MediaMTX service
func Start() error {
	loadEnv()
	unitName := "frameflow-mediamtx.service"

	isServer := os.Getenv("FRAMEFLOW_ROLE") == "SERVER"
	var userEnv []string
	var systemctlArgs []string
	var journalctlArgs []string

	if isServer {
		userEnv = nil
		systemctlArgs = []string{"systemctl"}
		journalctlArgs = []string{"journalctl"}
	} else {
		userEnv = getUserEnv()
		systemctlArgs = []string{"systemctl", "--user"}
		journalctlArgs = []string{"journalctl", "--user"}
	}

	// Check if active
	output, _ := runCommandWithEnv(10*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "is-active", "--quiet", unitName)...)
	if output == "" {
		_, err := runCommandWithEnv(10*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "is-active", "--quiet", unitName)...)
		if err == nil {
			fmt.Println("[INFO] MediaMTX service is already running.")
			runCommandWithEnv(10*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "status", unitName, "--no-pager")...)
			return fmt.Errorf("service already running")
		}
	}

	fmt.Println("[INFO] Starting MediaMTX via Systemd...")
	_, err := runCommandWithEnv(30*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "start", unitName)...)

	if err != nil {
		fmt.Println("[ERR] Failed to start MediaMTX.")
		runCommandWithEnv(10*time.Second, userEnv, journalctlArgs[0], append(journalctlArgs[1:], "-u", unitName, "-n", "10", "--no-pager")...)
		return err
	}

	time.Sleep(2 * time.Second)
	_, err = runCommandWithEnv(10*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "is-active", "--quiet", unitName)...)
	if err == nil {
		fmt.Println("[OK] MediaMTX started.")
		if isServer {
			fmt.Printf("Logs: journalctl -u %s -f\n", unitName)
		} else {
			fmt.Printf("Logs: journalctl --user -u %s -f\n", unitName)
		}
	} else {
		fmt.Println("[ERR] Failed to start MediaMTX.")
		out, _ := runCommandWithEnv(10*time.Second, userEnv, journalctlArgs[0], append(journalctlArgs[1:], "-u", unitName, "-n", "10", "--no-pager")...)
		fmt.Println(out)
	}

	return nil
}

// Stop stops the MediaMTX service
func Stop() error {
	loadEnv()
	unitName := "frameflow-mediamtx.service"

	isServer := os.Getenv("FRAMEFLOW_ROLE") == "SERVER"
	var userEnv []string
	var systemctlArgs []string

	if isServer {
		userEnv = nil
		systemctlArgs = []string{"systemctl"}
	} else {
		userEnv = getUserEnv()
		systemctlArgs = []string{"systemctl", "--user"}
	}

	fmt.Println("Stopping MediaMTX...")
	_, err := runCommandWithEnv(30*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "stop", unitName)...)
	if err != nil {
		return err
	}

	fmt.Println("Stopped.")
	return nil
}

// Status shows the status of MediaMTX service
func Status() string {
	loadEnv()
	unitName := "frameflow-mediamtx.service"

	isServer := os.Getenv("FRAMEFLOW_ROLE") == "SERVER"
	var userEnv []string
	var systemctlArgs []string

	if isServer {
		userEnv = nil
		systemctlArgs = []string{"systemctl"}
	} else {
		userEnv = getUserEnv()
		systemctlArgs = []string{"systemctl", "--user"}
	}

	_, err := runCommandWithEnv(10*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "is-active", "--quiet", unitName)...)
	if err != nil {
		// Not active or crashed
		runCommandWithEnv(10*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "stop", unitName)...)
		runCommandWithEnv(10*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "kill", unitName)...)
	}

	out, _ := runCommandWithEnv(10*time.Second, userEnv, systemctlArgs[0], append(systemctlArgs[1:], "status", unitName, "--no-pager")...)
	return out
}

func getInstalledUser() string {
	// Simple mock of get_installed_user
	user := os.Getenv("FRAMEFLOW_USER")
	if user != "" {
		return user
	}
	out, err := runCommandWithEnv(10*time.Second, nil, "bash", "-c", "ls -ld /opt/VLX_FrameFlow | awk '{print $3}'")
	if err == nil && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out)
	}
	return "root"
}

func GenerateService() error {
	loadEnv()
	mediaMtxDir := os.Getenv("MEDIAMTX_DIR")
	if mediaMtxDir == "" {
		mediaMtxDir = "/opt/mediamtx"
	}
	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}

	isServer := os.Getenv("FRAMEFLOW_ROLE") == "SERVER"

	execPath := filepath.Join(mediaMtxDir, "mediamtx")
	configPath := filepath.Join(vlxSuiteDir, "etc", "mediamtx.settings")

	var serviceContent string

	if isServer {
		user := getInstalledUser()
		serviceContent = fmt.Sprintf(`[Unit]
Description=VLX FrameFlow MediaMTX Server
After=network.target

[Service]
Type=exec
User=%s
Group=%s
ExecStart=%s %s
Restart=always

[Install]
WantedBy=multi-user.target
`, user, user, execPath, configPath)
	} else {
		serviceContent = fmt.Sprintf(`[Unit]
Description=VLX FrameFlow MediaMTX Server
After=network.target

[Service]
Type=exec
ExecStart=%s %s
Restart=always

[Install]
WantedBy=default.target
`, execPath, configPath)
	}

	serviceDir := os.Getenv("TEST_SERVICE_DIR")
	if serviceDir == "" {
		if isServer {
			serviceDir = "/etc/systemd/system"
		} else {
			home, errHome := os.UserHomeDir()
			if errHome != nil {
				sysutils.Error("Failed to get user home directory: %v", errHome)
				return errHome
			}
			serviceDir = filepath.Join(home, ".config", "systemd", "user")
		}
	}

	err := os.MkdirAll(serviceDir, 0755)
	if err != nil {
		sysutils.Error("Failed to create systemd directory: %v", err)
		return err
	}

	servicePath := filepath.Join(serviceDir, "frameflow-mediamtx.service")
	err = os.WriteFile(servicePath, []byte(serviceContent), 0644)
	if err != nil {
		sysutils.Error("Failed to write MediaMTX service file: %v", err)
		return err
	}

	sysutils.Info("MediaMTX systemd service generated at %s", servicePath)
	return nil
}
