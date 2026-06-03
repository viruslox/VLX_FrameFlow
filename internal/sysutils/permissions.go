package sysutils

import (
	"fmt"
	"os"
	"os/user"
	"strings"
)

// CheckPermissions verifies if the current execution context has the required privileges.
// It checks if the effective UID is root (0), or if the user has a proper FrameFlow configuration.
// By default, it checks the current process's effective UID. You can pass a string to mock or
// explicitly check a specific UID (e.g., "1000").
func CheckPermissions(uidStr ...string) error {
	var effectiveUID string
	if len(uidStr) > 0 && uidStr[0] != "" {
		effectiveUID = uidStr[0]
	} else {
		effectiveUID = fmt.Sprintf("%d", os.Geteuid())
	}

	// If root, we're good
	if effectiveUID == "0" {
		return nil
	}

	// Get current user information
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("[ERR]: Requirement: Root privileges OR a fully configured user setup.\nPlease launch again this script as root")
	}

	username := currentUser.Username

	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}

	userProfile := fmt.Sprintf("%s/etc/frameflow.settings", vlxSuiteDir)

	sudoersFile := os.Getenv("SUDOERS_FILE")
	if sudoersFile == "" {
		sudoersFile = fmt.Sprintf("/etc/sudoers.d/90-%s", username)
	}

	// Check if user profile exists
	if _, err := os.Stat(userProfile); err == nil {
		// Check if sudoers file exists
		if _, err := os.Stat(sudoersFile); err == nil {
			return nil
		}

		// Or if sudo -n -l 2>/dev/null | grep -q "$VLXsuite_DIR/bin/VLX_FrameFlow"
		out, err := RunCommand(5000000000, "sudo", "-n", "-l") // 5 seconds
		if err == nil {
			if strings.Contains(out, fmt.Sprintf("%s/bin/VLX_FrameFlow", vlxSuiteDir)) {
				return nil
			}
		}
	}

	// Output error and return
	fmt.Println("[ERR]: Requirement: Root privileges OR a fully configured user setup.")
	fmt.Println("If You are allowed to, try: 'sudo passwd root' , then login as root")
	return fmt.Errorf("Please launch again this script as root")
}
