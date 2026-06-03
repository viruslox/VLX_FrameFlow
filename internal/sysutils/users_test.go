package sysutils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupServiceUser(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "frameflow-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("VLXsuite_DIR", tempDir)
	os.Setenv("FRAMEFLOW_ROLE", "CLIENT")
	defer os.Unsetenv("VLXsuite_DIR")
	os.Setenv("MEDIAMTX_DIR", filepath.Join(tempDir, "mediamtx"))
	defer os.Unsetenv("MEDIAMTX_DIR")

	t.Run("Invalid Username", func(t *testing.T) {
		err := SetupServiceUser("bad/user", false)
		if err == nil {
			t.Errorf("Expected error for invalid username")
		}
	})

	// Since SetupServiceUser makes system calls like adduser, usermod, chown, loginctl,
	// executing them successfully during automated tests requires root and actual system binaries.
	// As we don't necessarily have these mockable easily in Go without interfaces,
	// we test the input validation heavily here. The full bash script has mocked adduser/usermod.
}

func TestSetupSudoUser(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "frameflow-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("VLXsuite_DIR", tempDir)
	os.Setenv("FRAMEFLOW_ROLE", "CLIENT")
	defer os.Unsetenv("VLXsuite_DIR")

	t.Run("Happy Path", func(t *testing.T) {
		err := SetupSudoUser("testuser", tempDir)
		if err != nil {
			t.Fatalf("Expected nil error, got %v", err)
		}

		expectedFile := filepath.Join(tempDir, "90-testuser")
		contentBytes, err := os.ReadFile(expectedFile)
		if err != nil {
			t.Fatalf("Failed to read expected file: %v", err)
		}
		content := string(contentBytes)

		expectedLines := []string{
			"testuser ALL=(ALL) NOPASSWD: " + tempDir + "/bin/VLX_FrameFlow",
		}

		for _, line := range expectedLines {
			if !strings.Contains(content, line) {
				t.Errorf("Expected content to contain '%s'", line)
			}
		}

		info, err := os.Stat(expectedFile)
		if err != nil {
			t.Fatalf("Failed to stat file: %v", err)
		}
		if info.Mode().Perm() != 0440 {
			t.Errorf("Expected permission 0440, got %v", info.Mode().Perm())
		}
	})

	t.Run("Missing Directory", func(t *testing.T) {
		subDir := filepath.Join(tempDir, "subdir")
		err := SetupSudoUser("testuser", subDir)
		if err != nil {
			t.Fatalf("Expected nil error, got %v", err)
		}
		if _, err := os.Stat(subDir); os.IsNotExist(err) {
			t.Errorf("Expected directory %s to be created", subDir)
		}
	})

	t.Run("Invalid User", func(t *testing.T) {
		err := SetupSudoUser("bad/user", tempDir)
		if err == nil {
			t.Errorf("Expected error for invalid username")
		}
	})

	t.Run("Root User", func(t *testing.T) {
		err := SetupSudoUser("root", tempDir)
		if err == nil {
			t.Errorf("Expected error for root user")
		}
	})

	t.Run("Server Role", func(t *testing.T) {
		os.Setenv("FRAMEFLOW_ROLE", "SERVER")
		defer os.Unsetenv("FRAMEFLOW_ROLE")
		err := SetupSudoUser("testuser", tempDir)
		if err == nil {
			t.Errorf("Expected error for server role")
		}
	})
}
