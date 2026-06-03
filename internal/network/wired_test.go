package network

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateNetworkdProfile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "frameflow-test-wired-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	err = GenerateNetworkdProfile("eth1", "20", tempDir)
	if err != nil {
		t.Fatalf("GenerateNetworkdProfile failed: %v", err)
	}

	filePath := filepath.Join(tempDir, "10-eth1.network")
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read network file: %v", err)
	}
	content := string(contentBytes)

	if !strings.Contains(content, "Name=eth1") {
		t.Errorf("Expected Name=eth1, got:\n%s", content)
	}
	if !strings.Contains(content, "Table=20") {
		t.Errorf("Expected Table=20, got:\n%s", content)
	}
	if !strings.Contains(content, "RouteMetric=120") {
		t.Errorf("Expected RouteMetric=120, got:\n%s", content)
	}
	if !strings.Contains(content, "DHCP=yes") {
		t.Errorf("Expected DHCP=yes, got:\n%s", content)
	}
}

func TestGenerateDispatcherUpScript(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "frameflow-test-wired-up-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	err = GenerateDispatcherUpScript("eth0", "10", tempDir)
	if err != nil {
		t.Fatalf("GenerateDispatcherUpScript failed: %v", err)
	}

	filePath := filepath.Join(tempDir, "30-eth0-mptcp-subflow.sh")
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read up script: %v", err)
	}
	content := string(contentBytes)

	if !strings.Contains(content, "IFACE\" = \"eth0\"") {
		t.Errorf("Expected IFACE=eth0, got:\n%s", content)
	}
	if !strings.Contains(content, "lookup 10") {
		t.Errorf("Expected lookup 10, got:\n%s", content)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("Expected permissions 0755, got %v", info.Mode().Perm())
	}
}

func TestGenerateDispatcherDownScript(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "frameflow-test-wired-down-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	err = GenerateDispatcherDownScript("eth0", "10", tempDir)
	if err != nil {
		t.Fatalf("GenerateDispatcherDownScript failed: %v", err)
	}

	filePath := filepath.Join(tempDir, "30-eth0-mptcp-stop.sh")
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read down script: %v", err)
	}
	content := string(contentBytes)

	if !strings.Contains(content, "IFACE\" = \"eth0\"") {
		t.Errorf("Expected IFACE=eth0, got:\n%s", content)
	}
	if !strings.Contains(content, "lookup 10") {
		t.Errorf("Expected lookup 10, got:\n%s", content)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("Expected permissions 0755, got %v", info.Mode().Perm())
	}
}
