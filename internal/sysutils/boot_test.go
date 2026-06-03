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

	os.MkdirAll(filepath.Join(tempMnt, "boot", "firmware"), 0755)
	os.MkdirAll(filepath.Join(tempMnt, "boot", "efi"), 0755)
	os.MkdirAll(filepath.Join(tempMnt, "etc"), 0755)

	targetDev := "/dev/sdX"

	t.Run("Standard Replacement cmdline.txt", func(t *testing.T) {
		cmdlinePath := filepath.Join(tempMnt, "boot", "efi", "cmdline.txt")
		os.WriteFile(cmdlinePath, []byte("console=serial0,115200 root=PARTUUID=abcd-1234 rootfstype=ext4"), 0644)

		err := ConfigureBoot(targetDev, tempMnt)
		if err != nil {
			t.Fatalf("ConfigureBoot failed: %v", err)
		}

		if _, err := os.Stat(cmdlinePath + ".BK"); os.IsNotExist(err) {
			t.Errorf("Backup file was not created")
		}

		content, _ := os.ReadFile(cmdlinePath)
		expected := "console=serial0,115200 root=UUID=UUID-PART-4 rootfstype=ext4"
		if strings.TrimSpace(string(content)) != expected {
			t.Errorf("Expected '%s', got '%s'", expected, string(content))
		}
	})

	t.Run("Fallback Overwrite cmdline.txt", func(t *testing.T) {
		cmdlinePath := filepath.Join(tempMnt, "boot", "efi", "cmdline.txt")
		os.WriteFile(cmdlinePath, []byte("console=serial0,115200 someotherparam=value rootfstype=ext4"), 0644)

		err := ConfigureBoot(targetDev, tempMnt)
		if err != nil {
			t.Fatalf("ConfigureBoot failed: %v", err)
		}

		content, _ := os.ReadFile(cmdlinePath)
		expected := "console=serial0,115200 console=tty1 root=UUID=UUID-PART-4 rootfstype=ext4 fsck.repair=yes rootwait nosplash debug --verbose cfg80211.ieee80211_regdom=IT consoleblank=0"
		if strings.TrimSpace(string(content)) != expected {
			t.Errorf("Expected '%s', got '%s'", expected, string(content))
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

	t.Run("Missing cmdline.txt", func(t *testing.T) {
		os.Remove(filepath.Join(tempMnt, "boot", "efi", "cmdline.txt"))
		os.Remove(filepath.Join(tempMnt, "boot", "efi", "cmdline.txt.BK"))

		err := ConfigureBoot(targetDev, tempMnt)
		if err != nil {
			t.Fatalf("ConfigureBoot failed when cmdline missing: %v", err)
		}

		if _, err := os.Stat(filepath.Join(tempMnt, "boot", "efi", "cmdline.txt.BK")); err == nil {
			t.Errorf("Backup file should not be created")
		}

		if _, err := os.Stat(filepath.Join(tempMnt, "etc", "fstab")); os.IsNotExist(err) {
			t.Errorf("fstab was not generated")
		}
	})
}
