package mediamtx

import (
	"fmt"
	"os"
	"path/filepath"
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

	archSuffix := "linux_arm64.tar.gz"
	if os.Getenv("FRAMEFLOW_ROLE") == "SERVER" {
		archSuffix = "linux_amd64.tar.gz"
	}

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

			user := getInstalledUser()
			_, err = runCommandWithEnv(10*time.Second, nil, "chown", fmt.Sprintf("%s:%s", user, user), mediaMtxDir, mediaMtxPath)
			if err != nil {
				return err
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

	sysutils.Success("MediaMTX ready.")
	return nil
}

// Start starts the MediaMTX service via systemd-run
func Start() error {
	loadEnv()
	unitName := "frameflow-mediamtx"
	mediaMtxDir := os.Getenv("MEDIAMTX_DIR")
	if mediaMtxDir == "" {
		mediaMtxDir = "/opt/mediamtx"
	}
	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}
	templateFile := filepath.Join(vlxSuiteDir, "etc", "mediamtx.settings")

	// Check if active
	output, _ := runCommandWithEnv(10*time.Second, nil, "systemctl", "--user", "is-active", "--quiet", unitName)
	if output == "" {
		// systemctl is-active returns empty output and 0 exit code if active, non-zero if inactive. Wait, the runCommand func will return err if exit code != 0.
		// Let's actually check it properly:
		_, err := runCommandWithEnv(10*time.Second, nil, "systemctl", "--user", "is-active", "--quiet", unitName)
		if err == nil {
			fmt.Println("[INFO] MediaMTX service is already running.")
			runCommandWithEnv(10*time.Second, nil, "systemctl", "--user", "status", unitName, "--no-pager")
			return fmt.Errorf("service already running")
		}
	}

	if _, err := os.Stat(templateFile); os.IsNotExist(err) {
		fmt.Printf("[ERR] Template file not found at: %s\n", templateFile)
		return fmt.Errorf("template file not found")
	}

	fmt.Println("[INFO] Starting MediaMTX via Systemd...")
	_, err := runCommandWithEnv(30*time.Second, nil, "systemd-run", "--user",
		"--unit="+unitName,
		"--description=VLX FrameFlow MediaMTX Server",
		"--collect",
		"--property=Restart=on-failure",
		"--property=RestartSec=5",
		"--service-type=exec",
		filepath.Join(mediaMtxDir, "mediamtx"), templateFile)

	if err != nil {
		fmt.Println("[ERR] Failed to start MediaMTX.")
		runCommandWithEnv(10*time.Second, nil, "journalctl", "--user", "-u", unitName, "-n", "10", "--no-pager")
		return err
	}

	time.Sleep(2 * time.Second)
	_, err = runCommandWithEnv(10*time.Second, nil, "systemctl", "--user", "is-active", "--quiet", unitName)
	if err == nil {
		fmt.Println("[OK] MediaMTX started.")
		fmt.Printf("Logs: journalctl --user -u %s -f\n", unitName)
	} else {
		fmt.Println("[ERR] Failed to start MediaMTX.")
		out, _ := runCommandWithEnv(10*time.Second, nil, "journalctl", "--user", "-u", unitName, "-n", "10", "--no-pager")
		fmt.Println(out)
	}

	return nil
}

// Stop stops the MediaMTX service
func Stop() error {
	loadEnv()
	unitName := "frameflow-mediamtx"
	fmt.Println("Stopping MediaMTX...")
	_, err := runCommandWithEnv(30*time.Second, nil, "systemctl", "--user", "stop", unitName)
	if err != nil {
		return err
	}

	fmt.Println("Stopped.")
	return nil
}

// Status shows the status of MediaMTX service
func Status() error {
	loadEnv()
	unitName := "frameflow-mediamtx"

	_, err := runCommandWithEnv(10*time.Second, nil, "systemctl", "--user", "is-active", "--quiet", unitName)
	if err != nil {
		// Not active or crashed
		runCommandWithEnv(10*time.Second, nil, "systemctl", "--user", "stop", unitName)
		runCommandWithEnv(10*time.Second, nil, "pkill", "-u", os.Getenv("USER"), "mediamtx")
	}

	out, err := runCommandWithEnv(10*time.Second, nil, "systemctl", "--user", "status", unitName, "--no-pager")
	fmt.Print(out)
	return err
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
