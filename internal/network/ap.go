package network

import (
	"fmt"
	"os"

	"strings"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

func SystemAccesspointStart() error {
	sysutils.Info("Starting Access Point...")

	systemdNetDir := os.Getenv("SYSTEMD_NETWORK")
	if systemdNetDir == "" {
		systemdNetDir = "/etc/systemd/network"
	}
	apProfileDir := os.Getenv("AP_PROFILE")
	if apProfileDir == "" {
		apProfileDir = "/etc/systemd/network/profiles/ap-bonding"
	}

	wifiIf, err := GetFirstWifiInterface("/sys/class/net")
	if err != nil || wifiIf == "" {
		sysutils.Error("No wireless interface found to start AP.")
		return fmt.Errorf("no wireless interface")
	}

	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "stop", "hostapd"); err != nil {
		return fmt.Errorf("failed to stop hostapd: %w, output: %s", err, out)
	}
	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "stop", fmt.Sprintf("wpa_supplicant@%s.service", wifiIf)); err != nil {
		return fmt.Errorf("failed to stop wpa_supplicant: %w, output: %s", err, out)
	}
	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "disable", "hostapd"); err != nil {
		return fmt.Errorf("failed to disable hostapd: %w, output: %s", err, out)
	}
	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "disable", fmt.Sprintf("wpa_supplicant@%s.service", wifiIf)); err != nil {
		return fmt.Errorf("failed to disable wpa_supplicant: %w, output: %s", err, out)
	}

	os.Remove(fmt.Sprintf("%s/20-%s-managed.network", systemdNetDir, wifiIf))

	if out, err := sysutils.RunCommand(10*time.Second, "cp", fmt.Sprintf("%s/40-%s-ap.network", apProfileDir, wifiIf), systemdNetDir+"/"); err != nil {
		return fmt.Errorf("failed to cp network profile: %w, output: %s", err, out)
	}

	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "restart", "systemd-networkd.service"); err != nil {
		return fmt.Errorf("failed to restart systemd-networkd: %w, output: %s", err, out)
	}

	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "restart", "systemd-resolved.service"); err != nil {
		return fmt.Errorf("failed to restart systemd-resolved: %w, output: %s", err, out)
	}

	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "unmask", "hostapd"); err != nil {
		return fmt.Errorf("failed to unmask hostapd: %w, output: %s", err, out)
	}
	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "enable", "hostapd"); err != nil {
		return fmt.Errorf("failed to enable hostapd: %w, output: %s", err, out)
	}
	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "start", "hostapd"); err != nil {
		return fmt.Errorf("failed to start hostapd: %w, output: %s", err, out)
	}

	sysutils.Success("Access Point Mode: Enabled")
	return nil
}

func SystemAccesspointStop() error {
	sysutils.Info("Stopping Access Point...")

	systemdNetDir := os.Getenv("SYSTEMD_NETWORK")
	if systemdNetDir == "" {
		systemdNetDir = "/etc/systemd/network"
	}

	wifiIf, err := GetFirstWifiInterface("/sys/class/net")
	if err != nil || wifiIf == "" {
		sysutils.Error("No wireless interface found to stop AP.")
		return fmt.Errorf("no wireless interface")
	}

	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "stop", "hostapd"); err != nil {
		return fmt.Errorf("failed to stop hostapd: %w, output: %s", err, out)
	}
	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "disable", "hostapd"); err != nil {
		return fmt.Errorf("failed to disable hostapd: %w, output: %s", err, out)
	}
	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "mask", "hostapd"); err != nil {
		return fmt.Errorf("failed to mask hostapd: %w, output: %s", err, out)
	}

	os.Remove(fmt.Sprintf("%s/40-%s-ap.network", systemdNetDir, wifiIf))

	normProfileDir := os.Getenv("NORM_PROFILE")
	if normProfileDir == "" {
		normProfileDir = "/etc/systemd/network/profiles/normal"
	}
	if out, err := sysutils.RunCommand(10*time.Second, "sh", "-c", fmt.Sprintf("cp %s/*.network %s/", normProfileDir, systemdNetDir)); err != nil {
		return fmt.Errorf("failed to cp managed network profile (from %s to %s): %w, output: %s", normProfileDir, systemdNetDir, err, out)
	}

	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "restart", "systemd-networkd.service"); err != nil {
		return fmt.Errorf("failed to restart systemd-networkd: %w, output: %s", err, out)
	}

	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "restart", "systemd-resolved.service"); err != nil {
		return fmt.Errorf("failed to restart systemd-resolved: %w, output: %s", err, out)
	}

	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "unmask", fmt.Sprintf("wpa_supplicant@%s.service", wifiIf)); err != nil {
		return fmt.Errorf("failed to unmask wpa_supplicant: %w, output: %s", err, out)
	}
	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "enable", fmt.Sprintf("wpa_supplicant@%s.service", wifiIf)); err != nil {
		return fmt.Errorf("failed to enable wpa_supplicant: %w, output: %s", err, out)
	}
	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "start", fmt.Sprintf("wpa_supplicant@%s.service", wifiIf)); err != nil {
		return fmt.Errorf("failed to start wpa_supplicant: %w, output: %s", err, out)
	}

	sysutils.Success("Access Point Mode: Disabled")
	return nil
}

func SystemAccesspointStatus() string {
	sysutils.Info("Checking Access Point status...")

	wifiIf, err := GetFirstWifiInterface("/sys/class/net")
	if err != nil || wifiIf == "" {
		sysutils.Error("No wireless interface found.")
		return "inactive"
	}

	hostapdActive := false
	out, err := sysutils.RunCommand(5*time.Second, "systemctl", "is-active", "hostapd")
	if err == nil && strings.TrimSpace(out) == "active" {
		hostapdActive = true
	}

	out, _ = sysutils.RunCommand(5*time.Second, "ip", "-o", "link", "show", wifiIf)
	if !strings.Contains(out, "UP") {
		sysutils.Warning("Interface %s is down, attempting recovery...", wifiIf)
		sysutils.RunCommand(10*time.Second, "ip", "link", "set", wifiIf, "up")
	}

	if hostapdActive {
		sysutils.Success("Access Point Mode: Enabled")
		return "active"
	} else {
		sysutils.Success("Access Point Mode: Disabled (Managed Client)")
		return "inactive"
	}
}

func AccesspointStart() error {
	binary, err := os.Executable()
	if err != nil {
		binary = "VLX_FrameFlow"
	}
	out, err := sysutils.RunCommand(30*time.Second, "sudo", binary, "ap", "_ap_system_ops", "start")
	if err != nil {
		return fmt.Errorf("failed to start AP: %w, output: %s", err, out)
	}
	return nil
}

func AccesspointStop() error {
	binary, err := os.Executable()
	if err != nil {
		binary = "VLX_FrameFlow"
	}
	out, err := sysutils.RunCommand(30*time.Second, "sudo", binary, "ap", "_ap_system_ops", "stop")
	if err != nil {
		return fmt.Errorf("failed to stop AP: %w, output: %s", err, out)
	}
	return nil
}

func AccesspointStatus() error {
	binary, err := os.Executable()
	if err != nil {
		binary = "VLX_FrameFlow"
	}
	out, err := sysutils.RunCommand(30*time.Second, "sudo", binary, "ap", "_ap_system_ops", "status")
	if err != nil {
		return fmt.Errorf("failed to get AP status: %w, output: %s", err, out)
	}
	return nil
}
