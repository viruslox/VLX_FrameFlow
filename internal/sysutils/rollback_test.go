package sysutils

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestCleanupSystemConfiguration(t *testing.T) {
	oldRunCommand := execCommandContext
	defer func() { execCommandContext = oldRunCommand }()

	execCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		cmdStr := command + " " + strings.Join(args, " ")
		if !strings.Contains(cmdStr, "crontab") {
			t.Errorf("Unexpected command: %s", cmdStr)
		}
		return exec.CommandContext(ctx, "echo", "mocked")
	}

	err := CleanupSystemConfiguration()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestCleanupServerConfiguration(t *testing.T) {
	oldRunCommand := execCommandContext
	defer func() { execCommandContext = oldRunCommand }()

	var calls []string
	execCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		calls = append(calls, command+" "+strings.Join(args, " "))
		return exec.CommandContext(ctx, "echo", "mocked")
	}

	err := CleanupServerConfiguration()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	foundStop := false
	for _, call := range calls {
		if strings.Contains(call, "systemctl stop frameflow-mptcp-proxy.service") {
			foundStop = true
		}
	}
	if !foundStop {
		t.Fatalf("Expected stop service to be called")
	}
}

func TestCleanupNetworkConfiguration(t *testing.T) {
	oldRunCommand := execCommandContext
	defer func() { execCommandContext = oldRunCommand }()

	var calls []string
	execCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		calls = append(calls, command+" "+strings.Join(args, " "))
		return exec.CommandContext(ctx, "echo", "mocked")
	}

	err := CleanupNetworkConfiguration()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	foundApt := false
	for _, call := range calls {
		if strings.Contains(call, "apt-get purge -y systemd-resolved networkd-dispatcher") {
			foundApt = true
		}
	}
	if !foundApt {
		t.Fatalf("Expected apt-get purge to be called")
	}
}
