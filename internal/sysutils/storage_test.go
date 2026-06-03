package sysutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Helper to create a dummy command that echoes its arguments instead of executing.
func dummyCommand(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// TestHelperProcess isn't a real test, it's used as a dummy executable.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stdout, "No command")
		os.Exit(0)
	}

	cmd := args[0]

	switch cmd {
	case "findmnt":
		fmt.Fprintln(os.Stdout, "/dev/nvme0n1p2")
	case "lsblk":
		if strings.Contains(strings.Join(args, " "), "-s /dev/nvme0n1p2") {
			fmt.Fprintln(os.Stdout, "nvme0n1p2\nnvme0n1")
		} else {
			fmt.Fprintln(os.Stdout, "/dev/nvme0n1\n/dev/nvme1n1")
		}
	case "sfdisk", "partprobe", "mkfs.vfat", "mkfs.ext4", "mkswap", "mount":
		// Success
	default:
		// Unknown command
		fmt.Fprintf(os.Stderr, "Unknown command: %s", cmd)
		os.Exit(1)
	}

	os.Exit(0)
}

func setupMocks() func() {
	oldRunCommand := RunCommand
	oldExecCommandContext := execCommandContext

	RunCommand = func(timeout time.Duration, name string, args ...string) (string, error) {
		cmd := dummyCommand(context.Background(), name, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), fmt.Errorf("mock RunCommand failed: %w", err)
		}
		return string(out), nil
	}
	execCommandContext = dummyCommand

	return func() {
		RunCommand = oldRunCommand
		execCommandContext = oldExecCommandContext
	}
}

func TestListStorageDevices(t *testing.T) {
	defer setupMocks()()

	devices, err := ListStorageDevices()
	if err != nil {
		t.Fatalf("ListStorageDevices failed: %v", err)
	}

	// Given our mock:
	// findmnt returns /dev/nvme0n1p2
	// lsblk -s returns nvme0n1p2, nvme0n1
	// lsblk block devices returns /dev/nvme0n1 and /dev/nvme1n1
	// Should exclude nvme0n1, return nvme1n1
	if len(devices) != 1 || devices[0] != "/dev/nvme1n1" {
		t.Errorf("Expected [/dev/nvme1n1], got %v", devices)
	}
}

func TestPartitionDriveGPT(t *testing.T) {
	defer setupMocks()()

	err := PartitionDriveGPT("/dev/testdev")
	if err != nil {
		t.Errorf("PartitionDriveGPT failed: %v", err)
	}
}

func TestFormatPartitions(t *testing.T) {
	defer setupMocks()()

	err := FormatPartitions("/dev/testdev", false)
	if err != nil {
		t.Errorf("FormatPartitions failed: %v", err)
	}

	err = FormatPartitions("/dev/testdev", true)
	if err != nil {
		t.Errorf("FormatPartitions skipHome failed: %v", err)
	}
}

func TestPrepareMounts(t *testing.T) {
	defer setupMocks()()

	tempDir := t.TempDir()
	mnt := fmt.Sprintf("%s/mnt", tempDir)

	err := PrepareMounts("/dev/testdev", false, mnt)
	if err != nil {
		t.Errorf("PrepareMounts failed: %v", err)
	}

	// Verify the base directories were created
	if _, err := os.Stat(mnt); os.IsNotExist(err) {
		t.Error("Expected mount directory to be created")
	}
	if _, err := os.Stat(filepath.Join(mnt, "boot", "efi")); os.IsNotExist(err) {
		t.Error("Expected boot/efi directory to be created")
	}
	if _, err := os.Stat(filepath.Join(mnt, "home")); os.IsNotExist(err) {
		t.Error("Expected home directory to be created")
	}
}
