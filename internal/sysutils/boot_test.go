package sysutils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureBoot(t *testing.T) {
	tempMnt, err := os.MkdirTemp("", "frameflow-test-boot-*")
	if err != nil {
		t.Fatalf("Failed to create temp mnt: %v", err)
	}
	defer os.RemoveAll(tempMnt)

	os.MkdirAll(filepath.Join(tempMnt, "boot", "extlinux"), 0755)
	os.MkdirAll(filepath.Join(tempMnt, "boot", "grub"), 0755)
	os.MkdirAll(filepath.Join(tempMnt, "boot", "efi"), 0755)
	os.MkdirAll(filepath.Join(tempMnt, "etc"), 0755)

	targetDev := "/dev/nvme0n1"

	t.Run("Standard Replacement extlinux.conf", func(t *testing.T) {
		extlinuxPath := filepath.Join(tempMnt, "boot", "extlinux", "extlinux.conf")
		os.WriteFile(extlinuxPath, []byte("    append console=ttyS2,1500000 root=PARTUUID=abcd-1234 rootfstype=ext4"), 0644)

		err := ConfigureBoot(targetDev, tempMnt)
		if err != nil {
			t.Fatalf("ConfigureBoot failed: %v", err)
		}

		if _, err := os.Stat(extlinuxPath + ".BK"); os.IsNotExist(err) {
			t.Errorf("Backup file was not created")
		}

		content, _ := os.ReadFile(extlinuxPath)
		expected := "append console=ttyS2,1500000 root=UUID=UUID-PART-4 rootfstype=ext4"
		if strings.TrimSpace(string(content)) != expected {
			t.Errorf("Expected '%s', got '%s'", expected, strings.TrimSpace(string(content)))
		}
	})

	t.Run("Fallback Overwrite extlinux.conf", func(t *testing.T) {
		extlinuxPath := filepath.Join(tempMnt, "boot", "extlinux", "extlinux.conf")
		os.WriteFile(extlinuxPath, []byte("    append console=ttyS2,1500000 someotherparam=value rootfstype=ext4"), 0644)

		err := ConfigureBoot(targetDev, tempMnt)
		if err != nil {
			t.Fatalf("ConfigureBoot failed: %v", err)
		}

		content, _ := os.ReadFile(extlinuxPath)
		expected := "label Debian\n    kernel /vmlinuz\n    initrd /initrd.img\n    append console=ttyS2,1500000 console=tty1 root=UUID=UUID-PART-4 rootfstype=ext4 fsck.repair=yes rootwait nosplash cfg80211.ieee80211_regdom=IT consoleblank=0"
		if strings.TrimSpace(string(content)) != expected {
			t.Errorf("Expected '%s', got '%s'", expected, strings.TrimSpace(string(content)))
		}
	})

	t.Run("Standard Replacement grub.cfg", func(t *testing.T) {
		grubPath := filepath.Join(tempMnt, "boot", "grub", "grub.cfg")
		os.WriteFile(grubPath, []byte("linux /vmlinuz root=UUID=old-uuid-1234 ro quiet"), 0644)

		err := ConfigureBoot(targetDev, tempMnt)
		if err != nil {
			t.Fatalf("ConfigureBoot failed: %v", err)
		}

		if _, err := os.Stat(grubPath + ".BK"); os.IsNotExist(err) {
			t.Errorf("GRUB backup file was not created")
		}

		content, _ := os.ReadFile(grubPath)
		expected := "linux /vmlinuz root=UUID=UUID-PART-4 ro quiet"
		if strings.TrimSpace(string(content)) != expected {
			t.Errorf("Expected '%s', got '%s'", expected, strings.TrimSpace(string(content)))
		}
	})

	t.Run("Correct fstab Generation", func(t *testing.T) {
		fstabPath := filepath.Join(tempMnt, "etc", "fstab")
		content, _ := os.ReadFile(fstabPath)

		expected := `proc /proc proc defaults 0 0
UUID=UUID-PART-1 /boot/efi vfat defaults 0 2
UUID=UUID-PART-2 /boot ext4 defaults 0 2
UUID=UUID-PART-4 / ext4 errors=remount-ro 0 1
UUID=UUID-PART-5 /home ext4 defaults 0 2
UUID=UUID-PART-3 none swap sw 0 0`

		if strings.TrimSpace(string(content)) != expected {
			t.Errorf("Expected fstab:\n%s\nGot:\n%s", expected, string(content))
		}
	})

	t.Run("Missing boot configs", func(t *testing.T) {
		os.Remove(filepath.Join(tempMnt, "boot", "extlinux", "extlinux.conf"))
		os.Remove(filepath.Join(tempMnt, "boot", "extlinux", "extlinux.conf.BK"))
		os.Remove(filepath.Join(tempMnt, "boot", "grub", "grub.cfg"))
		os.Remove(filepath.Join(tempMnt, "boot", "grub", "grub.cfg.BK"))

		err := ConfigureBoot(targetDev, tempMnt)
		if err != nil {
			t.Fatalf("ConfigureBoot failed when boot configs missing: %v", err)
		}

		if _, err := os.Stat(filepath.Join(tempMnt, "boot", "extlinux", "extlinux.conf.BK")); err == nil {
			t.Errorf("Backup file should not be created")
		}

		if _, err := os.Stat(filepath.Join(tempMnt, "etc", "fstab")); os.IsNotExist(err) {
			t.Errorf("fstab was not generated")
		}
	})
}
