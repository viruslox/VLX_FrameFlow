package sysutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

var execCommandContext = exec.CommandContext

func ensureDBusEnv(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	hasXdg := false
	hasDbus := false
	hasXauth := false
	for _, e := range env {
		if len(e) > 15 && e[:16] == "XDG_RUNTIME_DIR=" {
			hasXdg = true
		}
		if len(e) > 24 && e[:25] == "DBUS_SESSION_BUS_ADDRESS=" {
			hasDbus = true
		}
		if len(e) > 10 && e[:11] == "XAUTHORITY=" {
			hasXauth = true
		}
	}
	uidStr := os.Getenv("SUDO_UID")
	uid := os.Getuid()
	if uidStr != "" {
		if parsed, err := strconv.Atoi(uidStr); err == nil {
			uid = parsed
		}
	}

	if !hasXdg {
		env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=/run/user/%d", uid))
	}
	if !hasDbus {
		env = append(env, fmt.Sprintf("DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/%d/bus", uid))
	}
	if !hasXauth {
		env = append(env, fmt.Sprintf("XAUTHORITY=/run/user/%d/gdm/Xauthority", uid))
	}
	return env
}

func injectMachineArg(name string, args []string) []string {
	if name != "systemctl" && name != "systemd-run" {
		return args
	}

	if os.Geteuid() != 0 {
		return args
	}

	hasUser := false
	for _, arg := range args {
		if arg == "--user" {
			hasUser = true
			break
		}
	}

	if !hasUser {
		return args
	}

	uidStr := os.Getenv("SUDO_UID")
	uid := os.Getuid()
	if uidStr != "" {
		if parsed, err := strconv.Atoi(uidStr); err == nil {
			uid = parsed
		}
	}

	machineArg := fmt.Sprintf("--machine=%d@.host", uid)

	var newArgs []string
	for _, arg := range args {
		if arg == "--user" {
			newArgs = append(newArgs, machineArg)
		}
		newArgs = append(newArgs, arg)
	}
	return newArgs
}

// RunCommand executes a command with a timeout.
var RunCommand = func(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args = injectMachineArg(name, args)
	cmd := execCommandContext(ctx, name, args...)
	cmd.Env = ensureDBusEnv(nil)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("command timed out after %v", timeout)
	}

	if err != nil {
		return string(output), fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

// RunCommandWithEnv executes a command with a timeout and custom environment variables.
var RunCommandWithEnv = func(timeout time.Duration, env []string, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args = injectMachineArg(name, args)
	cmd := execCommandContext(ctx, name, args...)
	cmd.Env = ensureDBusEnv(env)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("command timed out after %v", timeout)
	}

	if err != nil {
		return string(output), fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}
