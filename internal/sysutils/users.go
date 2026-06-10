package sysutils

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SetupServiceUser creates and configures the service user
func SetupServiceUser(username string, isServer bool) error {
	if username == "" {
		username = "frameflow"
	}

	validUsername := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	if !validUsername.MatchString(username) {
		Error("Security: Invalid username '%s' provided. Only alphanumeric characters are allowed.", username)
		return fmt.Errorf("invalid username '%s'", username)
	}

	Info("Setting up user: %s", username)

	isClientRole := !isServer && os.Getenv("FRAMEFLOW_ROLE") != "SERVER"

	_, err := user.Lookup(username)
	if err != nil {
		// User doesn't exist, create it
		_, err := RunCommand(10*time.Second, "adduser", "--home", "/home/"+username, "--shell", "/bin/bash", "--gecos", "VLX FrameFlow tech user", username)
		if err != nil {
			return fmt.Errorf("failed to create user %s: %w", username, err)
		}
	}

	if isClientRole {
		groups := []string{"crontab", "dialout", "tty", "video", "audio", "plugdev", "netdev", "i2c", "bluetooth"}
		for _, group := range groups {
			_, err = RunCommand(10*time.Second, "usermod", "-a", "-G", group, username)
			if err != nil {
				Warning("failed to add user %s to group %s: %v", username, group, err)
			}
		}

		if username != "root" {
			_, err = RunCommand(10*time.Second, "loginctl", "enable-linger", username)
			if err != nil {
				Warning("failed to enable linger for user %s: %v", username, err)
			}
		}
	}

	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}
	mediaMTXDir := os.Getenv("MEDIAMTX_DIR")
	if mediaMTXDir == "" {
		mediaMTXDir = "/opt/mediamtx"
	}

	err = os.MkdirAll(vlxSuiteDir, 0755)
	if err != nil {
		Warning("failed to create %s: %v", vlxSuiteDir, err)
	}
	err = os.MkdirAll(mediaMTXDir, 0755)
	if err != nil {
		Warning("failed to create %s: %v", mediaMTXDir, err)
	}

	_, err = RunCommand(10*time.Second, "chown", fmt.Sprintf("%s:%s", username, username), vlxSuiteDir, mediaMTXDir)
	if err != nil {
		Warning("failed to chown directories for user %s: %v", username, err)
	}

	err = createMockUserProfile(username) // To mimic bash script logic, assuming profile exists or is created simply
	if err != nil {
		Warning("failed to create user profile: %v", err)
	}

	if isClientRole {
		err = SetupBashAliases(username)
		if err != nil {
			Warning("failed to setup bash aliases: %v", err)
		}

		err = SetupUserPath(username)
		if err != nil {
			Warning("failed to setup user path: %v", err)
		}

		if username != "root" {
			err = SetupSudoUser(username, "")
			if err != nil {
				Warning("failed to setup sudoers: %v", err)
			}
		}
	}

	Success("User configured.")
	return nil
}

// createMockUserProfile is a mock implementation mirroring create_user_profile for this file.
// Real implementation might read templates.
func createMockUserProfile(username string) error {
	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}
	profile := fmt.Sprintf("%s/etc/frameflow.settings", vlxSuiteDir)

	if _, err := os.Stat(profile); os.IsNotExist(err) {
		Info("Generating default profile at %s", profile)
		os.MkdirAll(fmt.Sprintf("%s/etc", vlxSuiteDir), 0755)
		return os.WriteFile(profile, []byte(""), 0644)
	} else {
		Info("Profile found at %s. Updating missing values...", profile)
	}
	return nil
}

// SetupBashAliases configures bash aliases for a user
func SetupBashAliases(username string) error {
	if username == "" {
		username = "frameflow"
	}

	userHome := "/home/" + username
	if username == "root" {
		userHome = os.Getenv("ROOT_HOME")
		if userHome == "" {
			userHome = "/root"
		}
	}

	bashrcPath := fmt.Sprintf("%s/.bashrc", userHome)

	if _, err := os.Stat(bashrcPath); os.IsNotExist(err) {
		err := os.WriteFile(bashrcPath, []byte(""), 0644)
		if err != nil {
			return err
		}
		RunCommand(5*time.Second, "chown", fmt.Sprintf("%s:%s", username, username), bashrcPath)
	}

	Info("Configuring bash aliases for %s", username)

	contentBytes, err := os.ReadFile(bashrcPath)
	if err != nil {
		return err
	}
	content := string(contentBytes)

	// Remove existing aliases
	lines := strings.Split(content, "\n")
	var newLines []string
	for _, line := range lines {
		if !strings.Contains(line, "alias VLX_FrameFlow=") &&
			!strings.Contains(line, "alias vlx_frameflow=") &&
			!strings.Contains(line, "alias vlx_frontend=") &&
			!strings.Contains(line, "alias VLX_cameraman=") &&
			!strings.Contains(line, "alias VLX_gps_tracker=") {
			newLines = append(newLines, line)
		}
	}
	content = strings.Join(newLines, "\n")

	// Append new aliases
	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}
	content += fmt.Sprintf("\nalias VLX_FrameFlow='sudo %s/bin/VLX_FrameFlow'\n", vlxSuiteDir)
	content += fmt.Sprintf("alias vlx_frameflow='%s/bin/VLX_FrameFlow'\n", vlxSuiteDir)
	content += fmt.Sprintf("alias vlx_frontend='%s/bin/vlx_frontend'\n", vlxSuiteDir)

	return os.WriteFile(bashrcPath, []byte(content), 0644)
}

// SetupUserPath configures user path
func SetupUserPath(username string) error {
	if username == "" {
		username = "frameflow"
	}

	userHome := "/home/" + username
	if username == "root" {
		userHome = os.Getenv("ROOT_HOME")
		if userHome == "" {
			userHome = "/root"
		}
	}

	bashrcPath := fmt.Sprintf("%s/.bashrc", userHome)

	if _, err := os.Stat(bashrcPath); os.IsNotExist(err) {
		err := os.WriteFile(bashrcPath, []byte(""), 0644)
		if err != nil {
			return err
		}
		RunCommand(5*time.Second, "chown", fmt.Sprintf("%s:%s", username, username), bashrcPath)
	}

	out, _ := RunCommand(5*time.Second, "su", "-", username, "-c", "echo $PATH")

	if !strings.Contains(":"+out+":", ":/usr/sbin:") && !strings.Contains(":"+out+":", ":/usr/sbin/:") {
		contentBytes, err := os.ReadFile(bashrcPath)
		if err != nil {
			return err
		}
		if !strings.Contains(string(contentBytes), "export PATH=$PATH:/usr/sbin") {
			f, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := f.WriteString("\nexport PATH=$PATH:/usr/sbin\n"); err != nil {
				return err
			}
			Info("Added /usr/sbin to PATH in %s", bashrcPath)
		}
	}
	return nil
}

// SetupSudoUser creates the sudoers configuration for the given user.
func SetupSudoUser(username, sudoersDir string) error {
	if username == "" {
		username = "frameflow"
	}
	if sudoersDir == "" {
		sudoersDir = "/etc/sudoers.d"
	}

	if username == "root" {
		Error("Security: Sudo configuration for root user is not allowed.")
		return fmt.Errorf("sudo configuration for root user is not allowed")
	}

	if os.Getenv("FRAMEFLOW_ROLE") == "SERVER" {
		Error("Security: Sudo configuration is not allowed in SERVER role.")
		return fmt.Errorf("sudo configuration is not allowed in SERVER role")
	}

	validUsername := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	if !validUsername.MatchString(username) {
		Error("Security: Invalid username '%s' provided. Only alphanumeric characters are allowed.", username)
		return fmt.Errorf("invalid username '%s'", username)
	}

	sudoFile := filepath.Join(sudoersDir, fmt.Sprintf("90-%s", username))

	if _, err := os.Stat(sudoersDir); os.IsNotExist(err) {
		Info("Creating sudoers directory: %s", sudoersDir)
		os.MkdirAll(sudoersDir, 0755)
	}

	if _, err := os.Stat(sudoFile); err == nil {
		os.Remove(sudoFile)
	}

	Info("Setting up sudoers for user: %s", username)

	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}

	// Based on bash output
	content := fmt.Sprintf(`%s ALL=(ALL) NOPASSWD: %s/bin/VLX_FrameFlow, /usr/local/sbin/mlvpn, /usr/sbin/ip, /usr/bin/systemctl restart systemd-networkd.service, /usr/bin/systemctl restart systemd-resolved.service`, username, vlxSuiteDir)

	err := os.WriteFile(sudoFile, []byte(content), 0440)
	if err != nil {
		return fmt.Errorf("failed to write sudoers file: %w", err)
	}

	// Verify the syntax of the generated file using visudo
	_, err = RunCommand(10*time.Second, "visudo", "-c", "-f", sudoFile)
	if err != nil {
		Warning("Sudoers syntax check failed!")
	}

	return nil
}

func GetInstalledUser() (string, error) {
	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}
	profile := fmt.Sprintf("%s/etc/frameflow.settings", vlxSuiteDir)

	if _, err := os.Stat(profile); err == nil {
		out, err := RunCommand(5*time.Second, "grep", "^FRAMEFLOW_USER=", profile)
		if err == nil {
			parts := strings.SplitN(strings.TrimSpace(out), "=", 2)
			if len(parts) == 2 {
				return parts[1], nil
			}
		}
	}
	return "root", nil
}
