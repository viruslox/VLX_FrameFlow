package sysutils

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	EfiSystemGuid = "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
	LinuxFsGuid   = "0FC63DAF-8483-4772-8E79-3D69D8477DE4"
	LinuxSwapGuid = "0657FD6D-A4AB-43C4-84E5-0933C84B4F4F"

	PartNumEfi  = 1
	PartNumBoot = 2
	PartNumSwap = 3
	PartNumRoot = 4
	PartNumHome = 5
)

// ListStorageDevices lists available NVMe/MMC block devices excluding the root device
func ListStorageDevices() ([]string, error) {
	// 1. Get root partition
	rootPartOut, err := RunCommand(5*time.Second, "findmnt", "-n", "-o", "SOURCE", "-e", "/")
	if err != nil {
		return nil, nil // Return empty if findmnt fails (fail safe)
	}
	rootPart := strings.TrimSpace(rootPartOut)
	if rootPart == "" {
		return nil, nil
	}

	// 2. Get hierarchy of devices for the root partition
	hierarchyOut, err := RunCommand(5*time.Second, "lsblk", "-n", "-o", "NAME", "-s", rootPart)
	var excludePattern string
	if err == nil {
		hierarchyLines := strings.Split(strings.TrimSpace(hierarchyOut), "\n")
		var hierarchyClean []string
		for _, line := range hierarchyLines {
			cleanLine := strings.TrimSpace(line)
			if cleanLine != "" {
				hierarchyClean = append(hierarchyClean, cleanLine)
			}
		}
		if len(hierarchyClean) > 0 {
			excludePattern = strings.Join(hierarchyClean, "|")
		}
	}

	// Fallback to name if lsblk -s fails or is empty
	if excludePattern == "" {
		parts := strings.Split(rootPart, "/")
		rootName := parts[len(parts)-1]
		if rootName != "" {
			excludePattern = rootName
		} else {
			return nil, nil
		}
	}

	// 3. List block devices
	out, err := RunCommand(5*time.Second, "lsblk", "-p", "-d", "-n", "-o", "NAME")
	if err != nil {
		return nil, nil
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Match nvme or mmcblk
		if strings.Contains(line, "nvme") || strings.Contains(line, "mmcblk") {
			// Check if excluded
			excluded := false
			parts := strings.Split(excludePattern, "|")
			for _, part := range parts {
				if strings.HasSuffix(line, "/"+part) {
					excluded = true
					break
				}
			}
			if !excluded {
				result = append(result, line)
			}
		}
	}

	return result, nil
}

// PartitionDriveGPT creates a GPT partition layout on the target device
func PartitionDriveGPT(dev string) error {
	Warning("Partitioning %s (GPT Layout)...", dev)

	sfdiskInput := fmt.Sprintf(`label: gpt
size=1G,type=%s,name="EFI System"
size=1G,type=%s,name="Linux boot"
size=4G,type=%s,name="Linux swap"
size=44G,type=%s,name="Linux root"
type=%s,name="Linux home"
`, EfiSystemGuid, LinuxFsGuid, LinuxSwapGuid, LinuxFsGuid, LinuxFsGuid)

	cmd := execCommandContext(context.Background(), "sfdisk", dev)
	cmd.Stdin = strings.NewReader(sfdiskInput)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sfdisk failed: %w", err)
	}

	_, err := RunCommand(1*time.Minute, "partprobe", dev)
	if err != nil {
		return fmt.Errorf("partprobe failed: %w", err)
	}

	time.Sleep(2 * time.Second)
	return nil
}

// FormatPartitions formats the partitions on the target device
func FormatPartitions(dev string, skipHome bool) error {
	Info("Formatting partitions...")

	_, err := RunCommand(5*time.Minute, "mkfs.vfat", "-F", "32", "-n", "EFI", fmt.Sprintf("%sp%d", dev, PartNumEfi))
	if err != nil {
		return fmt.Errorf("Failed to format EFI: %w", err)
	}

	_, err = RunCommand(5*time.Minute, "mkfs.ext4", "-F", "-L", "boot", fmt.Sprintf("%sp%d", dev, PartNumBoot))
	if err != nil {
		return fmt.Errorf("Failed to format Boot: %w", err)
	}

	_, err = RunCommand(5*time.Minute, "mkswap", "-f", "-L", "swap", fmt.Sprintf("%sp%d", dev, PartNumSwap))
	if err != nil {
		return fmt.Errorf("Failed to format Swap: %w", err)
	}

	_, err = RunCommand(5*time.Minute, "mkfs.ext4", "-F", "-L", "root", fmt.Sprintf("%sp%d", dev, PartNumRoot))
	if err != nil {
		return fmt.Errorf("Failed to format Root: %w", err)
	}

	if !skipHome {
		_, err = RunCommand(5*time.Minute, "mkfs.ext4", "-F", "-L", "home", fmt.Sprintf("%sp%d", dev, PartNumHome))
		if err != nil {
			return fmt.Errorf("Failed to format Home: %w", err)
		}
	}

	return nil
}

// PrepareMounts prepares mount points
func PrepareMounts(targetDev string, skipHome bool, mnt string) error {
	if err := os.MkdirAll(mnt, 0755); err != nil {
		return err
	}

	if _, err := RunCommand(1*time.Minute, "mount", fmt.Sprintf("%sp%d", targetDev, PartNumRoot), mnt); err != nil {
		return err
	}

	if err := os.MkdirAll(fmt.Sprintf("%s/boot", mnt), 0755); err != nil {
		return err
	}
	if _, err := RunCommand(1*time.Minute, "mount", fmt.Sprintf("%sp%d", targetDev, PartNumBoot), fmt.Sprintf("%s/boot", mnt)); err != nil {
		return err
	}

	if err := os.MkdirAll(fmt.Sprintf("%s/boot/efi", mnt), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(fmt.Sprintf("%s/boot/firmware", mnt), 0755); err != nil {
		return err
	}
	if _, err := RunCommand(1*time.Minute, "mount", fmt.Sprintf("%sp%d", targetDev, PartNumEfi), fmt.Sprintf("%s/boot/efi", mnt)); err != nil {
		return err
	}

	// Some distros use /boot/firmware for EFI partition too. Optional, so ignore error.
	RunCommand(1*time.Minute, "mount", fmt.Sprintf("%sp%d", targetDev, PartNumEfi), fmt.Sprintf("%s/boot/firmware", mnt))

	if !skipHome {
		if err := os.MkdirAll(fmt.Sprintf("%s/home", mnt), 0755); err != nil {
			return err
		}
		if _, err := RunCommand(1*time.Minute, "mount", fmt.Sprintf("%sp%d", targetDev, PartNumHome), fmt.Sprintf("%s/home", mnt)); err != nil {
			return err
		}
	}

	return nil
}
