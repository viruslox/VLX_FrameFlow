package sysutils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/security"
)

var installTargetDir = "/opt/VLX_FrameFlow/bin"
var installConfigDir = "/opt/VLX_FrameFlow/etc"
var profileScriptPath = "/etc/profile.d/vlx_frameflow.sh"

func copyFile(src, dst string, perm os.FileMode) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}
	return nil
}

func mergeConfigs(templatePath, targetPath string) error {
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}

	targetContent, err := os.ReadFile(targetPath)
	if err != nil {
		return err
	}

	templateStr := string(templateContent)
	targetStr := string(targetContent)

	targetLines := strings.Split(targetStr, "\n")
	targetMap := make(map[string]bool)
	for _, l := range targetLines {
		trimmed := strings.TrimSpace(l)
		if strings.Contains(trimmed, "=") && !strings.HasPrefix(trimmed, "#") {
			key := strings.SplitN(trimmed, "=", 2)[0]
			targetMap[key] = true
		} else if strings.HasPrefix(trimmed, "#") {
			targetMap[trimmed] = true
		}
	}

	var missing []string
	for _, l := range strings.Split(templateStr, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "=") && !strings.HasPrefix(trimmed, "#") {
			key := strings.SplitN(trimmed, "=", 2)[0]
			if !targetMap[key] {
				missing = append(missing, l)
			}
		} else if strings.HasPrefix(trimmed, "#") {
			if !targetMap[trimmed] {
				missing = append(missing, l)
			}
		}
	}

	if len(missing) > 0 {
		if !strings.HasSuffix(targetStr, "\n") && targetStr != "" {
			targetStr += "\n"
		}
		targetStr += strings.Join(missing, "\n") + "\n"
		err = os.WriteFile(targetPath, []byte(targetStr), 0644)
		if err != nil {
			return err
		}
		Info("Merged new missing configuration into %s", targetPath)
	}

	return nil
}

// InstallBinary copies the currently running executable and associated components to the target directories.
func InstallBinary(isServer bool) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return err
	}

	// Create directories
	if err := os.MkdirAll(installTargetDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin dir: %w", err)
	}
	if err := os.MkdirAll(installConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create etc dir: %w", err)
	}

	installCertsDir := filepath.Join(filepath.Dir(installTargetDir), "certs")
	if err := os.MkdirAll(installCertsDir, 0755); err != nil {
		return fmt.Errorf("failed to create certs dir: %w", err)
	}

	caCertPath := filepath.Join(installCertsDir, "ca.crt")
	caKeyPath := filepath.Join(installCertsDir, "ca.key")
	if err := security.EnsureLocalCA(caCertPath, caKeyPath); err != nil {
		Info("Warning: Failed to ensure local CA: %v", err)
	} else {
		caCertPEM, errCert := os.ReadFile(caCertPath)
		caKeyPEM, errKey := os.ReadFile(caKeyPath)
		if errCert == nil && errKey == nil {
			serverCertPath := filepath.Join(installCertsDir, "server.crt")
			serverKeyPath := filepath.Join(installCertsDir, "server.key")
			if _, err := os.Stat(serverCertPath); os.IsNotExist(err) {
				srvCert, srvKey, err := security.GenerateServerCert(caCertPEM, caKeyPEM)
				if err == nil {
					os.WriteFile(serverCertPath, srvCert, 0644)
					os.WriteFile(serverKeyPath, srvKey, 0600)
				}
			}

			clientCertPath := filepath.Join(installCertsDir, "frontend01.crt")
			clientKeyPath := filepath.Join(installCertsDir, "frontend01.key")
			if _, err := os.Stat(clientCertPath); os.IsNotExist(err) {
				cliCert, cliKey, err := security.GenerateClientCert(caCertPEM, caKeyPEM, "frontend01")
				if err == nil {
					os.WriteFile(clientCertPath, cliCert, 0644)
					os.WriteFile(clientKeyPath, cliKey, 0600)
				}
			}
		}
	}

	// Copy executing binary
	baseName := filepath.Base(exePath)
	targetName := baseName
	if strings.HasSuffix(targetName, "_amd64") {
		targetName = strings.TrimSuffix(targetName, "_amd64")
	} else if strings.HasSuffix(targetName, "_arm64") {
		targetName = strings.TrimSuffix(targetName, "_arm64")
	}
	if isServer {
		targetName = "VLX_FrameFlow_SRV"
	}
	targetPath := filepath.Join(installTargetDir, targetName)
	if err := copyFile(exePath, targetPath, 0755); err != nil {
		return fmt.Errorf("failed to copy main binary: %w", err)
	}
	Info("Binary successfully installed to %s", targetPath)

	// Explicitly copy required binaries (including frontend for both roles)
	srcDir := filepath.Dir(exePath)
	var requiredBinaries []string
	if isServer {
		requiredBinaries = []string{"vlx_frontend"}
	} else {
		requiredBinaries = []string{"VLX_FrameFlow", "vlx_frontend"}
	}

	for _, bin := range requiredBinaries {
		binPath := filepath.Join(srcDir, bin)
		if _, err := os.Stat(binPath); err == nil {
			tgt := filepath.Join(installTargetDir, bin)
			if err := copyFile(binPath, tgt, 0755); err == nil {
				Info("Required binary %s explicitly copied to %s", bin, tgt)
			}
		} else {
			// Try with arch suffix for the current OS/Arch if raw name not found
			sfx := "_" + runtime.GOARCH
			sfxPath := filepath.Join(srcDir, bin+sfx)
			if _, err := os.Stat(sfxPath); err == nil {
				tgt := filepath.Join(installTargetDir, bin)
				if err := copyFile(sfxPath, tgt, 0755); err == nil {
					Info("Required binary %s (from %s) explicitly copied to %s", bin, bin+sfx, tgt)
				}
			}
		}
	}

	// Try to copy configs from a config dir adjacent to the srcDir, or cwd
	configSearchPaths := []string{"config", filepath.Join(filepath.Dir(srcDir), "config")}

	for _, searchPath := range configSearchPaths {
		if _, err := os.Stat(searchPath); err == nil {
			// Copy template user settings if it exists
			var userTemplate, userTarget string
			if isServer {
				userTemplate = filepath.Join(searchPath, "frameflow_srv.settings.template")
				userTarget = filepath.Join(installConfigDir, "frameflow.settings")
			} else {
				userTemplate = filepath.Join(searchPath, "frameflow.settings.template")
				userTarget = filepath.Join(installConfigDir, "frameflow.settings")
			}

			if _, err := os.Stat(userTemplate); err == nil {
				if _, err := os.Stat(userTarget); os.IsNotExist(err) {
					copyFile(userTemplate, userTarget, 0644)
					Info("Copied settings template to etc")
				} else {
					mergeConfigs(userTemplate, userTarget)
				}
			}

			if !isServer {
				// Copy frontend template user settings if it exists
				frontendTemplate := filepath.Join(searchPath, "frontend.settings.template")
				frontendTarget := filepath.Join(installConfigDir, "frontend.settings")
				if _, err := os.Stat(frontendTemplate); err == nil {
					if _, err := os.Stat(frontendTarget); os.IsNotExist(err) {
						copyFile(frontendTemplate, frontendTarget, 0644)
						Info("Copied frontend.settings.template to etc")
					} else {
						mergeConfigs(frontendTemplate, frontendTarget)
					}
				}
			}

			// Copy mediamtx settings template to etc/mediamtx.settings if it exists
			var mtxConfig string
			if isServer {
				mtxConfig = filepath.Join(searchPath, "mediamtx_server.settings.template")
			} else {
				mtxConfig = filepath.Join(searchPath, "mediamtx_client.settings.template")
			}
			mtxTarget := filepath.Join(installConfigDir, "mediamtx.settings")
			if _, err := os.Stat(mtxConfig); err == nil {
				if _, err := os.Stat(mtxTarget); os.IsNotExist(err) {
					copyFile(mtxConfig, mtxTarget, 0644)
					Info("Copied %s to etc/mediamtx.settings", filepath.Base(mtxConfig))
				} else {
					mergeConfigs(mtxConfig, mtxTarget)
				}
			}
			break
		}
	}

	// Append or create profile path injection
	if profileScriptPath == "/etc/profile.d/vlx_frameflow.sh" || filepath.Base(profileScriptPath) == "vlx_frameflow.sh" {
		profileContent := fmt.Sprintf("export PATH=$PATH:%s\n", installTargetDir)
		err := os.WriteFile(profileScriptPath, []byte(profileContent), 0644)
		if err == nil {
			Info("Injected PATH into %s", profileScriptPath)
		} else {
			Info("Warning: Failed to inject PATH: %v", err)
		}
	}

	// Interactive User configuration
	eligibleUsersOutput, err := RunCommand(10*time.Second, "awk", "-F:", "{if ($3 >= 1000 && ($7 == \"/bin/bash\" || $7 == \"/bin/zsh\")) print $1}", "/etc/passwd")
	var eligibleUsers []string
	if err == nil {
		for _, u := range strings.Split(eligibleUsersOutput, "\n") {
			u = strings.TrimSpace(u)
			if u != "" {
				eligibleUsers = append(eligibleUsers, u)
			}
		}
	}

	fmt.Println("\nAvailable non-privileged users:")
	for _, u := range eligibleUsers {
		fmt.Printf("- %s\n", u)
	}
	fmt.Print("\nCreate dedicated user (default frameflow) or use an existing user: ")

	reader := bufio.NewReader(os.Stdin)
	selectedUser, _ := reader.ReadString('\n')
	selectedUser = strings.TrimSpace(selectedUser)
	if selectedUser == "" {
		selectedUser = "frameflow"
	}

	err = SetupServiceUser(selectedUser, isServer)
	if err != nil {
		Info("Warning: SetupServiceUser completed with errors: %v", err)
	}

	// Update FRAMEFLOW_USER in frameflow.settings
	settingsName := "frameflow.settings"
	settingsPath := filepath.Join(installConfigDir, settingsName)
	if _, err := os.Stat(settingsPath); err == nil {
		settingsContent, err := os.ReadFile(settingsPath)
		if err == nil {
			var newLines []string
			for _, line := range strings.Split(string(settingsContent), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "FRAMEFLOW_USER=") {
					newLines = append(newLines, fmt.Sprintf("FRAMEFLOW_USER=\"%s\"", selectedUser))
				} else {
					newLines = append(newLines, line)
				}
			}
			os.WriteFile(settingsPath, []byte(strings.Join(newLines, "\n")), 0644)
		}
	}

	_, err = RunCommand(10*time.Second, "chown", "-Rf", fmt.Sprintf("%s:%s", selectedUser, selectedUser), "/opt/VLX_FrameFlow/")
	if err != nil {
		Info("Warning: Failed to set ownership for /opt/VLX_FrameFlow/: %v", err)
	}

	Info("Calling mediamtx installer...")

	// Avoid import cycle by launching mediamtx installer through system exec if possible,
	// but to do it natively we could launch our own executable with a mediamtx flag.
	// We'll execute the mediamtx install function if we refactor it, but for now we'll do:
	// A new package might be needed, or we just invoke the command line.
	// Since mediamtx install can be called from client option 8, we can call it from here by running the currently running executable using cobra commands.
	_, err = RunCommand(10*time.Minute, "sudo", "-u", selectedUser, targetPath, "mediamtx", "install")
	if err != nil {
		Info("Warning: Failed to run mediamtx setup via CLI: %v", err)
	}

	_, err = RunCommand(10*time.Second, "chown", "-Rf", fmt.Sprintf("%s:%s", selectedUser, selectedUser), "/opt/mediamtx/")
	if err != nil {
		Info("Warning: Failed to set ownership for /opt/mediamtx/: %v", err)
	}

	return nil
}
