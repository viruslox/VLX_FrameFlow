package sysutils

import (
	"os"
	"testing"
)

func TestRestorePackagesValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "frameflow-test-pkg-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
}

func TestInstallDependenciesRole(t *testing.T) {
	t.Skip("skip")
}

func TestUpdateSuiteCode(t *testing.T) {
	t.Skip("skip")
}

func TestSetupMaintenanceCron(t *testing.T) {
	t.Skip("skip")
}

func TestInstallShadowsocks(t *testing.T) {
	t.Skip("skip")
}

func TestInstallMlvpn(t *testing.T) {
	t.Skip("skip")
}
