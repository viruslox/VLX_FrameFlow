package sysutils

import (
	"os"
	"strings"
	"testing"
)

func TestEnsureDBusEnv(t *testing.T) {
	// Keep original SUDO_UID
	origSudoUid := os.Getenv("SUDO_UID")
	defer func() {
		if origSudoUid == "" {
			os.Unsetenv("SUDO_UID")
		} else {
			os.Setenv("SUDO_UID", origSudoUid)
		}
	}()

	os.Setenv("SUDO_UID", "1001")

	env := ensureDBusEnv([]string{})

	hasXdg := false
	hasDbus := false
	for _, e := range env {
		if strings.HasPrefix(e, "XDG_RUNTIME_DIR=") {
			if e != "XDG_RUNTIME_DIR=/run/user/1001" {
				t.Errorf("Unexpected XDG_RUNTIME_DIR: %s", e)
			}
			hasXdg = true
		}
		if strings.HasPrefix(e, "DBUS_SESSION_BUS_ADDRESS=") {
			if e != "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus" {
				t.Errorf("Unexpected DBUS_SESSION_BUS_ADDRESS: %s", e)
			}
			hasDbus = true
		}
	}

	if !hasXdg {
		t.Errorf("Expected XDG_RUNTIME_DIR to be set")
	}
	if !hasDbus {
		t.Errorf("Expected DBUS_SESSION_BUS_ADDRESS to be set")
	}
}
