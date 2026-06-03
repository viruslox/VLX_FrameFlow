package network

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetFirstWifiInterface(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "frameflow-test-wifi-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test 1: No interfaces
	_, err = GetFirstWifiInterface(tempDir)
	if err == nil || !strings.Contains(err.Error(), "no wireless interface found") {
		t.Errorf("Expected 'no wireless interface found', got err: %v", err)
	}

	// Test 2: Only wired interfaces
	err = os.MkdirAll(filepath.Join(tempDir, "eth0"), 0755)
	if err != nil {
		t.Fatalf("Failed to create eth0 dir: %v", err)
	}
	err = os.MkdirAll(filepath.Join(tempDir, "enp3s0"), 0755)
	if err != nil {
		t.Fatalf("Failed to create enp3s0 dir: %v", err)
	}

	_, err = GetFirstWifiInterface(tempDir)
	if err == nil || !strings.Contains(err.Error(), "no wireless interface found") {
		t.Errorf("Expected 'no wireless interface found', got err: %v", err)
	}

	// Test 3: One wireless interface
	err = os.MkdirAll(filepath.Join(tempDir, "wlan0", "wireless"), 0755)
	if err != nil {
		t.Fatalf("Failed to create wlan0 dir: %v", err)
	}

	iface, err := GetFirstWifiInterface(tempDir)
	if err != nil {
		t.Errorf("Expected success, got err: %v", err)
	}
	if iface != "wlan0" {
		t.Errorf("Expected 'wlan0', got '%s'", iface)
	}

	// Test 4: Multiple wireless interfaces (should return first found based on OS iteration)
	err = os.MkdirAll(filepath.Join(tempDir, "wlan1", "wireless"), 0755)
	if err != nil {
		t.Fatalf("Failed to create wlan1 dir: %v", err)
	}

	iface, err = GetFirstWifiInterface(tempDir)
	if err != nil {
		t.Errorf("Expected success, got err: %v", err)
	}
	if iface != "wlan0" && iface != "wlan1" {
		t.Errorf("Expected 'wlan0' or 'wlan1', got '%s'", iface)
	}
}

func TestCreateAPProfile_DefaultDNS(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "frameflow-test-ap-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Ensure vars are unset
	os.Unsetenv("DNS_SERVERS")
	os.Unsetenv("DNS_SERVERS_V6")

	err = CreateAPProfile("test0", tempDir)
	if err != nil {
		t.Fatalf("CreateAPProfile failed: %v", err)
	}

	filePath := filepath.Join(tempDir, "40-test0-ap.network")
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read profile file: %v", err)
	}
	content := string(contentBytes)

	if !strings.Contains(content, "DNS=8.8.8.8 1.1.1.1") {
		t.Errorf("Expected default IPv4 DNS, got:\n%s", content)
	}

	if !strings.Contains(content, "DNS=2001:4860:4860::8888 2606:4700:4700::1111") {
		t.Errorf("Expected default IPv6 DNS, got:\n%s", content)
	}
}

func TestCreateAPProfile_CustomDNS(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "frameflow-test-ap-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("DNS_SERVERS", "9.9.9.9 1.0.0.1")
	os.Setenv("DNS_SERVERS_V6", "2606:4700:4700::1111 2001:db8::1")
	defer os.Unsetenv("DNS_SERVERS")
	defer os.Unsetenv("DNS_SERVERS_V6")

	err = CreateAPProfile("test1", tempDir)
	if err != nil {
		t.Fatalf("CreateAPProfile failed: %v", err)
	}

	filePath := filepath.Join(tempDir, "40-test1-ap.network")
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read profile file: %v", err)
	}
	content := string(contentBytes)

	if !strings.Contains(content, "DNS=9.9.9.9 1.0.0.1") {
		t.Errorf("Expected custom IPv4 DNS, got:\n%s", content)
	}

	if !strings.Contains(content, "DNS=2606:4700:4700::1111 2001:db8::1") {
		t.Errorf("Expected custom IPv6 DNS, got:\n%s", content)
	}
}

func TestCreateHostapdConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "frameflow-test-hostapd-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	err = CreateHostapdConfig("wlan0", tempDir, "mockhost", "mockpass")
	if err != nil {
		t.Fatalf("CreateHostapdConfig failed: %v", err)
	}

	filePath := filepath.Join(tempDir, "hostapd.conf")
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read hostapd file: %v", err)
	}
	content := string(contentBytes)

	expectedLines := []string{
		"interface=wlan0",
		"ssid=VLX_mockhost",
		"wpa_passphrase=mockpass",
		"driver=nl80211",
		"hw_mode=g",
		"channel=6",
		"wpa=2",
	}

	for _, line := range expectedLines {
		if !strings.Contains(content, line) {
			t.Errorf("Expected to find '%s' in hostapd.conf, but got:\n%s", line, content)
		}
	}
}

func TestCreateManagedProfile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "frameflow-test-managed-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "20-wlan0-managed.network")
	err = CreateManagedProfile("wlan0", filePath, 0)
	if err != nil {
		t.Fatalf("CreateManagedProfile failed: %v", err)
	}

	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read profile file: %v", err)
	}
	content := string(contentBytes)

	if !strings.Contains(content, "Name=wlan0") {
		t.Errorf("Expected Name=wlan0, got:\n%s", content)
	}
	if !strings.Contains(content, "RouteMetric=200") {
		t.Errorf("Expected RouteMetric=200, got:\n%s", content)
	}
	if !strings.Contains(content, "WPAConfigFile=/etc/wpa_supplicant/wpa_supplicant-wlan0.conf") {
		t.Errorf("Expected WPAConfigFile=/etc/wpa_supplicant/wpa_supplicant-wlan0.conf, got:\n%s", content)
	}
	if !strings.Contains(content, "WiFiPowerSave=disable") {
		t.Errorf("Expected WiFiPowerSave=disable, got:\n%s", content)
	}

	filePath2 := filepath.Join(tempDir, "20-wlan1-managed.network")
	err = CreateManagedProfile("wlan1", filePath2, 50)
	if err != nil {
		t.Fatalf("CreateManagedProfile failed: %v", err)
	}

	contentBytes2, err := os.ReadFile(filePath2)
	if err != nil {
		t.Fatalf("Failed to read profile file: %v", err)
	}
	content2 := string(contentBytes2)

	if !strings.Contains(content2, "Name=wlan1") {
		t.Errorf("Expected Name=wlan1, got:\n%s", content2)
	}
	if !strings.Contains(content2, "RouteMetric=250") {
		t.Errorf("Expected RouteMetric=250, got:\n%s", content2)
	}
	if !strings.Contains(content2, "WPAConfigFile=/etc/wpa_supplicant/wpa_supplicant-wlan1.conf") {
		t.Errorf("Expected WPAConfigFile=/etc/wpa_supplicant/wpa_supplicant-wlan1.conf, got:\n%s", content2)
	}
}
