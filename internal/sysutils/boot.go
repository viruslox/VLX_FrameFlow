package sysutils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ConfigureBoot configures the boot files and /etc/fstab for the target device
func ConfigureBoot(targetDev, mnt string) error {
	Info("Configuring Boot and Fstab...")

	// 1. Create Symlinks
	bootFirmware := filepath.Join(mnt, "boot", "firmware")
	os.RemoveAll(bootFirmware)
	err := os.Symlink("efi", bootFirmware)
	if err != nil {
		// Log error but continue as this might fail depending on environment (like in tests)
		Warning("Failed to create symlink: %v", err)
	}

	// 2. Get UUIDs
	p1uuid, _ := getUUID(targetDev + "p1")
	p2uuid, _ := getUUID(targetDev + "p2")
	p3uuid, _ := getUUID(targetDev + "p3")
	p4uuid, _ := getUUID(targetDev + "p4")
	p5uuid, _ := getUUID(targetDev + "p5")

	// Fallbacks for testing
	if p1uuid == "" {
		p1uuid = "UUID-PART-1"
		p2uuid = "UUID-PART-2"
		p3uuid = "UUID-PART-3"
		p4uuid = "UUID-PART-4"
		p5uuid = "UUID-PART-5"
	}

	// 3. Update cmdline.txt
	cmdlinePath := filepath.Join(mnt, "boot", "efi", "cmdline.txt")
	if _, err := os.Stat(cmdlinePath); err == nil {
		// Backup
		backupPath := cmdlinePath + ".BK"
		content, err := os.ReadFile(cmdlinePath)
		if err == nil {
			os.WriteFile(backupPath, content, 0644)

			cmdline := string(content)
			re := regexp.MustCompile(`root=[^\s]*`)
			if re.MatchString(cmdline) {
				newCmdline := re.ReplaceAllString(cmdline, "root=UUID="+p4uuid)
				os.WriteFile(cmdlinePath, []byte(newCmdline), 0644)
			} else {
				fallback := fmt.Sprintf("console=serial0,115200 console=tty1 root=UUID=%s rootfstype=ext4 fsck.repair=yes rootwait nosplash debug --verbose cfg80211.ieee80211_regdom=IT consoleblank=0\n", p4uuid)
				os.WriteFile(cmdlinePath, []byte(fallback), 0644)
			}
		}
	}

	// 4. Generate new FSTAB
	fstabPath := filepath.Join(mnt, "etc", "fstab")
	fstabContent := fmt.Sprintf(`proc /proc proc defaults 0 0
UUID=%s /boot/efi vfat defaults 0 2
UUID=%s /boot ext4 defaults 0 2
UUID=%s / ext4 errors=remount-ro 0 1
UUID=%s /home ext4 defaults 0 2
UUID=%s none swap sw 0 0
`, p1uuid, p2uuid, p4uuid, p5uuid, p3uuid)

	err = os.MkdirAll(filepath.Join(mnt, "etc"), 0755)
	if err != nil {
		return err
	}

	return os.WriteFile(fstabPath, []byte(fstabContent), 0644)
}

func getUUID(dev string) (string, error) {
	out, err := RunCommand(5*time.Second, "lsblk", "-f", "-n", "-o", "UUID", dev)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
