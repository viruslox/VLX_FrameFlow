package sysutils

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// RemoveBloatware removes packages that might interfere with FrameFlow
func RemoveBloatware(pkgs []string) error {
	Info("Removing packages that might interfere with FrameFlow")

	if len(pkgs) > 0 {
		args := append([]string{"purge", "-y"}, pkgs...)
		_, err := RunCommand(10*time.Minute, "apt-get", args...)
		if err != nil {
			return fmt.Errorf("apt-get purge failed: %w", err)
		}
	}

	_, err := RunCommand(5*time.Minute, "apt-get", "autoremove", "-y")
	if err != nil {
		return fmt.Errorf("apt-get autoremove failed: %w", err)
	}

	_, err = RunCommand(5*time.Minute, "aptitude", "-y", "purge", "~c")
	if err != nil {
		// Log but don't strictly fail as aptitude might not be installed
		Warning("aptitude purge ~c failed: %v", err)
	}

	Success("Cleaned.")
	return nil
}

// RestorePackages restores packages from a pkg.list file
func RestorePackages() error {
	pkgFile := "/root/pkg.list"

	if _, err := os.Stat(pkgFile); err == nil {
		Warning("Found a list of packages previously installed (%s).", pkgFile)

		// In a real CLI environment we would prompt the user. For now, we simulate "Yes" for automation or
		// assume this logic handles the validation part of reading the list.
		Info("Validating and restoring packages...")

		contentBytes, err := os.ReadFile(pkgFile)
		if err != nil {
			Error("Failed to read %s: %v", pkgFile, err)
			return err
		}

		content := string(contentBytes)
		lines := strings.Split(content, "\n")
		var pkgs []string

		validPkgRegex := regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]*(:[a-z0-9]+)?$`)

		for _, line := range lines {
			// Strip comments
			if idx := strings.Index(line, "#"); idx != -1 {
				line = line[:idx]
			}
			line = strings.TrimSpace(line)
			if line != "" && validPkgRegex.MatchString(line) {
				pkgs = append(pkgs, line)
			}
		}

		if len(pkgs) > 0 {
			args := append([]string{"install", "-y"}, pkgs...)
			_, err := RunCommand(10*time.Minute, "apt-get", args...)
			if err != nil {
				Error("Package restoration failed.")
				return fmt.Errorf("package restoration failed: %w", err)
			}
			Success("Packages restored successfully.")
		} else {
			Error("No valid packages found in %s.", pkgFile)
			return fmt.Errorf("no valid packages found in %s", pkgFile)
		}
	}

	return nil
}

// InstallDependencies installs system dependencies
func InstallDependencies(pkgsFirmware, pkgsSystem, pkgsAllInstall []string) error {
	Info("Installing dependencies")

	_, err := RunCommand(5*time.Minute, "apt", "--fix-broken", "install", "-y")
	if err != nil {
		return fmt.Errorf("apt --fix-broken install failed: %w", err)
	}

	role := os.Getenv("FRAMEFLOW_ROLE")
	var pkgsToInstall []string

	if role == "SERVER" {
		pkgsToInstall = append(pkgsToInstall, pkgsFirmware...)
		pkgsToInstall = append(pkgsToInstall, pkgsSystem...)
	} else {
		pkgsToInstall = append(pkgsToInstall, pkgsAllInstall...)
	}

	if len(pkgsToInstall) > 0 {
		args := append([]string{"install", "-y"}, pkgsToInstall...)
		_, err := RunCommand(10*time.Minute, "apt-get", args...)
		if err != nil {
			// Fallback: apt-get update && apt-get install
			_, updateErr := RunCommand(5*time.Minute, "apt-get", "update", "-y")
			if updateErr != nil {
				return fmt.Errorf("apt-get update failed: %w", updateErr)
			}
			_, retryErr := RunCommand(10*time.Minute, "apt-get", args...)
			if retryErr != nil {
				return fmt.Errorf("apt-get install failed after update: %w", retryErr)
			}
		}
	}

	Success("Installed dependencies.")
	return nil
}

func UpdateSuiteCode() error {
	Info("Updating source code...")

	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}
	os.MkdirAll(vlxSuiteDir, 0755)

	githubURL := "https://github.com/viruslox/VLX_FrameFlow.git"

	// Assume git is available and we just clone or pull
	if _, err := os.Stat(vlxSuiteDir + "/.git"); err == nil {
		RunCommand(5*time.Minute, "git", "-C", vlxSuiteDir, "reset", "--hard")
		RunCommand(5*time.Minute, "git", "-C", vlxSuiteDir, "pull", "--no-verify", githubURL)
	} else {
		RunCommand(5*time.Minute, "git", "clone", githubURL, vlxSuiteDir)
	}

	// Set permissions
	RunCommand(5*time.Second, "chmod", "700", vlxSuiteDir+"/bin/VLX_FrameFlow.sh")
	RunCommand(5*time.Second, "chmod", "700", vlxSuiteDir+"/config/FrameFlow_maintenance.sh")

	Success("Code updated.")
	return nil
}

func SetupMaintenanceCron() error {
	Info("Installing cron job...")
	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}
	cronScript := fmt.Sprintf("%s/config/FrameFlow_maintenance.sh", vlxSuiteDir)
	cronJob := fmt.Sprintf("@reboot %s start 2>&1", cronScript)

	out, _ := RunCommand(10*time.Second, "crontab", "-l")
	if !strings.Contains(out, cronScript) {
		tmpFile, _ := os.CreateTemp("", "cron")
		tmpFile.WriteString(out + "\n" + cronJob + "\n")
		tmpFile.Close()
		RunCommand(10*time.Second, "crontab", tmpFile.Name())
		os.Remove(tmpFile.Name())
		Success("Cron job added.")
	}
	return nil
}

func InstallShadowsocks() error {
	Info("Installing Shadowsocks...")

	out, _ := RunCommand(10*time.Second, "command", "-v", "ss-server")
	if out != "" {
		Success("Shadowsocks is already installed.")
		return nil
	}

	if _, err := RunCommand(10*time.Minute, "apt-get", "install", "-y", "shadowsocks-libev"); err == nil {
		Success("Shadowsocks installed via apt.")
		return nil
	}

	// Fallback to build
	args := []string{"install", "-y", "build-essential", "cmake", "pkg-config", "libpcre2-dev", "libev-dev", "libc-ares-dev", "libmbedtls-dev", "libsodium-dev", "asciidoc", "xmlto"}
	RunCommand(20*time.Minute, "apt-get", args...)

	buildDir := "/opt/shadowsocks"
	os.RemoveAll(buildDir)

	if _, err := RunCommand(10*time.Minute, "git", "clone", "--recursive", "https://github.com/shadowsocks/shadowsocks-libev.git", buildDir); err == nil {
		os.MkdirAll(buildDir+"/build", 0755)

		if _, err := RunCommand(10*time.Minute, "bash", "-c", "cd "+buildDir+"/build && cmake .."); err != nil {
			return err
		}

		if _, err := RunCommand(20*time.Minute, "bash", "-c", "cd "+buildDir+"/build && make"); err != nil {
			return err
		}

		if _, err := RunCommand(5*time.Minute, "bash", "-c", "cd "+buildDir+"/build && make install"); err == nil {
			Success("Shadowsocks installed successfully from source.")
			return nil
		}
	}

	return fmt.Errorf("failed to download and build Shadowsocks")
}

func InstallMlvpn() error {
	Info("Installing MLVPN...")

	out, _ := RunCommand(10*time.Second, "command", "-v", "mlvpn")
	if out != "" {
		Success("MLVPN is already installed.")
		return nil
	}

	if _, err := RunCommand(10*time.Minute, "apt-get", "update"); err != nil {
		return fmt.Errorf("failed to apt-get update: %w", err)
	}

	args := []string{"install", "-y", "build-essential", "pkg-config", "libsodium-dev", "libpcap-dev", "libev-dev", "wget", "tar"}
	if _, err := RunCommand(20*time.Minute, "apt-get", args...); err != nil {
		return fmt.Errorf("failed to install MLVPN dependencies: %w", err)
	}

	if _, err := RunCommand(5*time.Minute, "bash", "-c", "cd /tmp && wget https://github.com/zehome/MLVPN/releases/download/2.3.5/mlvpn-2.3.5.tar.gz -O mlvpn.tar.gz"); err != nil {
		return fmt.Errorf("failed to download MLVPN source: %w", err)
	}

	if _, err := RunCommand(5*time.Minute, "bash", "-c", "cd /tmp && tar -xzf mlvpn.tar.gz"); err != nil {
		return fmt.Errorf("failed to extract MLVPN source: %w", err)
	}

	if _, err := RunCommand(5*time.Minute, "bash", "-c", "cd /tmp/mlvpn-2.3.5 && ./configure"); err != nil {
		return fmt.Errorf("failed to configure MLVPN: %w", err)
	}

	if _, err := RunCommand(20*time.Minute, "bash", "-c", "cd /tmp/mlvpn-2.3.5 && make"); err != nil {
		return fmt.Errorf("failed to make MLVPN: %w", err)
	}

	if _, err := RunCommand(5*time.Minute, "bash", "-c", "cd /tmp/mlvpn-2.3.5 && make install"); err != nil {
		return fmt.Errorf("failed to install MLVPN: %w", err)
	}

	// Cleanup
	os.Remove("/tmp/mlvpn.tar.gz")
	os.RemoveAll("/tmp/mlvpn-2.3.5")

	Success("MLVPN installed successfully from source.")
	return nil
}
