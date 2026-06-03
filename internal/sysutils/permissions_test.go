package sysutils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckPermissions(t *testing.T) {
	// Setup a temporary directory to act as the home/opt directory for testing
	tempDir, err := os.MkdirTemp("", "frameflow-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	vlxSuiteDir := tempDir
	os.Setenv("VLXsuite_DIR", vlxSuiteDir)
	defer os.Unsetenv("VLXsuite_DIR")

	sudoersDir := filepath.Join(tempDir, "sudoers.d")
	os.MkdirAll(sudoersDir, 0755)

	configDir := filepath.Join(tempDir, "config")
	os.MkdirAll(configDir, 0755)

	// Test Case 1: Root user
	t.Run("Root user", func(t *testing.T) {
		err := CheckPermissions("0")
		if err != nil {
			t.Errorf("Expected nil error for root user, got: %v", err)
		}
	})

	// Test Case 2: Non-root user with profile and sudoers
	t.Run("Non-root user valid config", func(t *testing.T) {
		// Create profile and sudoers
		etcDir := filepath.Join(tempDir, "etc")
		os.MkdirAll(etcDir, 0755)
		userProfile := filepath.Join(etcDir, "frameflow.settings")
		os.WriteFile(userProfile, []byte(""), 0644)
		defer os.Remove(userProfile)

		sudoersFile := filepath.Join(sudoersDir, "90-testuser")
		os.WriteFile(sudoersFile, []byte(""), 0644)
		defer os.Remove(sudoersFile)

		os.Setenv("SUDOERS_FILE", sudoersFile)
		defer os.Unsetenv("SUDOERS_FILE")

		err := CheckPermissions("1000")
		if err != nil {
			t.Errorf("Expected nil error for valid config, got: %v", err)
		}
	})

	// Test Case 3: Non-root user, missing profile
	t.Run("Non-root user missing profile", func(t *testing.T) {
		sudoersFile := filepath.Join(sudoersDir, "90-testuser")
		os.WriteFile(sudoersFile, []byte(""), 0644)
		defer os.Remove(sudoersFile)

		os.Setenv("SUDOERS_FILE", sudoersFile)
		defer os.Unsetenv("SUDOERS_FILE")

		err := CheckPermissions("1000")
		if err == nil {
			t.Errorf("Expected error for missing profile, got nil")
		} else if err.Error() != "Please launch again this script as root" {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})

	// Test Case 4: Non-root user, missing sudoers
	t.Run("Non-root user missing sudoers", func(t *testing.T) {
		userProfile := filepath.Join(configDir, "frameflow.settings")
		os.WriteFile(userProfile, []byte(""), 0644)
		defer os.Remove(userProfile)

		os.Setenv("SUDOERS_FILE", filepath.Join(sudoersDir, "missing-file"))
		defer os.Unsetenv("SUDOERS_FILE")

		err := CheckPermissions("1000")
		if err == nil {
			t.Errorf("Expected error for missing sudoers, got nil")
		} else if err.Error() != "Please launch again this script as root" {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})

	// Test Case 5: Non-root user, missing both
	t.Run("Non-root user missing both", func(t *testing.T) {
		os.Setenv("SUDOERS_FILE", filepath.Join(sudoersDir, "missing-file"))
		defer os.Unsetenv("SUDOERS_FILE")

		err := CheckPermissions("1000")
		if err == nil {
			t.Errorf("Expected error for missing both, got nil")
		}
	})

	// Test Case 6: Non-root user, fallback to default SUDOERS_FILE
	t.Run("Non-root fallback to default SUDOERS_FILE", func(t *testing.T) {
		userProfile := filepath.Join(configDir, "frameflow.settings")
		os.WriteFile(userProfile, []byte(""), 0644)
		defer os.Remove(userProfile)

		os.Unsetenv("SUDOERS_FILE")

		// Since default /etc/sudoers.d/90-<user> doesn't exist in our controlled temp environment
		err := CheckPermissions("1000")
		if err == nil {
			t.Errorf("Expected error for default missing sudoers, got nil")
		}
	})
}
