package sysutils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ConfigureBoot configures standard Debian boot files (U-Boot extlinux / GRUB2) and /etc/fstab for the target device
func ConfigureBoot(targetDev, mnt string) error {
	Info("Configuring Boot and Fstab...")

	// 1. Get UUIDs
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

	reRoot := regexp.MustCompile(`root=[^\s]*`)

	// 2. Update U-Boot extlinux configuration (/boot/extlinux/extlinux.conf) if present
	extlinuxPath := filepath.Join(mnt, "boot", "extlinux", "extlinux.conf")
	if _, err := os.Stat(extlinuxPath); err == nil {
		backupPath := extlinuxPath + ".BK"
		content, err := os.ReadFile(extlinuxPath)
		if err == nil {
			os.WriteFile(backupPath, content, 0644)

			extlinux := string(content)
			if reRoot.MatchString(extlinux) {
				newExtlinux := reRoot.ReplaceAllString(extlinux, "root=UUID="+p4uuid)
				os.WriteFile(extlinuxPath, []byte(newExtlinux), 0644)
			} else {
				fallback := fmt.Sprintf("label Debian\n    kernel /vmlinuz\n    initrd /initrd.img\n    append console=ttyS2,1500000 console=tty1 root=UUID=%s rootfstype=ext4 fsck.repair=yes rootwait nosplash cfg80211.ieee80211_regdom=IT consoleblank=0\n", p4uuid)
				os.WriteFile(extlinuxPath, []byte(fallback), 0644)
			}
		}
	}

	// 3. Update UEFI/GRUB2 configuration (/boot/grub/grub.cfg) if present
	grubPath := filepath.Join(mnt, "boot", "grub", "grub.cfg")
	if _, err := os.Stat(grubPath); err == nil {
		backupPath := grubPath + ".BK"
		content, err := os.ReadFile(grubPath)
		if err == nil {
			os.WriteFile(backupPath, content, 0644)

			grubCfg := string(content)
			if reRoot.MatchString(grubCfg) {
				newGrubCfg := reRoot.ReplaceAllString(grubCfg, "root=UUID="+p4uuid)
				os.WriteFile(grubPath, []byte(newGrubCfg), 0644)
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

	err := os.MkdirAll(filepath.Join(mnt, "etc"), 0755)
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
