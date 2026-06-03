package sysutils

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// CleanupSystemConfiguration reverts system configuration by removing cron, sudoers and sysctl settings
func CleanupSystemConfiguration() error {
	Info("Reverting system configuration...")

	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}

	cronScript := fmt.Sprintf("%s/config/FrameFlow_maintenance.sh", vlxSuiteDir)

	out, err := RunCommand(10*time.Second, "crontab", "-l")
	if err == nil {
		lines := strings.Split(out, "\n")
		var newLines []string
		for _, line := range lines {
			if !strings.Contains(line, cronScript) {
				newLines = append(newLines, line)
			}
		}
		newCron := strings.Join(newLines, "\n")

		// Write to temp file and load
		tmpFile, _ := os.CreateTemp("", "cron")
		tmpFile.WriteString(newCron)
		tmpFile.Close()
		RunCommand(10*time.Second, "crontab", tmpFile.Name())
		os.Remove(tmpFile.Name())
		Info("Cron job removed.")
	}

	user, _ := GetInstalledUser()
	if user != "" && user != "root" {
		sudoFile := fmt.Sprintf("/etc/sudoers.d/90-%s", user)
		if _, err := os.Stat(sudoFile); err == nil {
			os.Remove(sudoFile)
			Info("Sudoers file removed for %s.", user)
		}
	}

	sysctlFile := "/etc/sysctl.d/99-disable-dmesg-restrict.conf"
	if _, err := os.Stat(sysctlFile); err == nil {
		os.Remove(sysctlFile)
	}

	Success("System configuration reverted.")
	return nil
}

func CleanupServerConfiguration() error {
	Info("Reverting server configuration...")

	services := []string{"frameflow-mptcp-proxy.service", "frameflow-mlvpn.service", "shadowsocks-libev.service"}
	for _, svc := range services {
		RunCommand(10*time.Second, "systemctl", "stop", svc)
		RunCommand(10*time.Second, "systemctl", "disable", svc)
	}

	os.Remove("/etc/systemd/system/frameflow-mptcp-proxy.service")
	os.Remove("/etc/systemd/system/frameflow-mlvpn.service")
	os.Remove("/usr/lib/systemd/system/shadowsocks-libev.service")

	RunCommand(10*time.Second, "systemctl", "daemon-reload")

	Success("Server configuration reverted.")
	return nil
}

func CleanupNetworkConfiguration() error {
	Info("Reverting network configuration...")

	services := []string{"systemd-networkd", "systemd-resolved", "networkd-dispatcher", "hostapd", "wpa_supplicant"}
	for _, svc := range services {
		RunCommand(10*time.Second, "systemctl", "stop", svc)
		RunCommand(10*time.Second, "systemctl", "disable", svc)
	}
	RunCommand(10*time.Second, "systemctl", "mask", "systemd-networkd")

	RunCommand(10*time.Minute, "apt-get", "purge", "-y", "systemd-resolved", "networkd-dispatcher")

	os.RemoveAll("/etc/systemd/network")
	os.RemoveAll("/etc/networkd-dispatcher")

	sysctlFile := "/etc/sysctl.d/97-forwarding.conf"
	if _, err := os.Stat(sysctlFile); err == nil {
		os.Remove(sysctlFile)
	}

	Success("Network configuration reverted.")
	return nil
}

func RunRollback() error {
	role := os.Getenv("FRAMEFLOW_ROLE")
	if role == "SERVER" {
		Warning("This will perform a complete cleanup of server components and configurations.")
		Warning("It will also remove the maintenance cron job and sudoers configuration.")
		Info("The tool installation directory and user profile will be preserved.")

		if AskConfirmation("Are you sure you want to proceed?") {
			CleanupServerConfiguration()

			if AskConfirmation("Do you want to remove the server firewall rules (ports 8889/tcp, 8322/tcp, 8189/udp, 8890/udp, 5080/udp, 8388)?") {
				Info("Removing UFW firewall rules...")
				RunCommand(10*time.Second, "ufw", "delete", "allow", "8889/tcp")
				RunCommand(10*time.Second, "ufw", "delete", "allow", "8322/tcp")
				RunCommand(10*time.Second, "ufw", "delete", "allow", "8189/udp")
				RunCommand(10*time.Second, "ufw", "delete", "allow", "8890/udp")
				RunCommand(10*time.Second, "ufw", "delete", "allow", "5080/udp")
				RunCommand(10*time.Second, "ufw", "delete", "allow", "8388")
				RunCommand(10*time.Second, "ufw", "delete", "allow", "out", "8889/tcp")
				RunCommand(10*time.Second, "ufw", "delete", "allow", "out", "8322/tcp")
				RunCommand(10*time.Second, "ufw", "delete", "allow", "out", "8189/udp")
				RunCommand(10*time.Second, "ufw", "delete", "allow", "out", "8890/udp")
				RunCommand(10*time.Second, "ufw", "delete", "allow", "out", "5080/udp")
				RunCommand(10*time.Second, "ufw", "delete", "allow", "out", "8388")
				RunCommand(10*time.Second, "ufw", "reload")
			}

			CleanupSystemConfiguration()
			Success("Rollback complete.")
		} else {
			Info("Rollback aborted.")
		}
	} else {
		Warning("This will perform a complete cleanup of network configurations, firewall rules, and routes.")
		Warning("It will also remove the maintenance cron job and sudoers configuration.")
		Info("The tool installation directory and user profile will be preserved.")

		if AskConfirmation("Are you sure you want to proceed?") {
			CleanupNetworkConfiguration()
			CleanupSystemConfiguration()
			Success("Rollback complete.")
		} else {
			Info("Rollback aborted.")
		}
	}
	return nil
}
