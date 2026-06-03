package telemetry

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	if os.Getenv("MOCK_EXIT_ERROR") == "1" {
		fmt.Fprintf(os.Stderr, "mock simulated error")
		os.Exit(1)
	}

	// os.Args is like: [binary, -test.run=TestHelperProcess, --, command, arg1, arg2, ...]
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command provided")
		os.Exit(2)
	}

	// Dump the command back to stdout for assertion
	fmt.Println(strings.Join(args, " "))
	os.Exit(0)
}

func TestExecutor_Run_Success(t *testing.T) {
	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	execCommand = fakeExecCommand

	executor := NewExecutor()

	out, err := executor.Run("my_script", "arg1", "arg2")

	assert.NoError(t, err)

	expectedOut := fmt.Sprintf("bash -lc shopt -s expand_aliases\nmy_script \"$@\" -- my_script arg1 arg2\n")
	assert.Equal(t, expectedOut, out)
}

func TestExecutor_Run_InvalidScript(t *testing.T) {
	executor := NewExecutor()

	_, err := executor.Run("invalid script name")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid script name")

	_, err = executor.Run("script&injection")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid script name")
}

func TestExecutor_Run_ExecutionError(t *testing.T) {
	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	execCommand = func(command string, args ...string) *exec.Cmd {
		cmd := fakeExecCommand(command, args...)
		cmd.Env = append(cmd.Env, "MOCK_EXIT_ERROR=1")
		return cmd
	}

	executor := NewExecutor()
	_, err := executor.Run("my_script")
	assert.Error(t, err)
}

func TestExecutor_Run_NoArgs(t *testing.T) {
	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	execCommand = fakeExecCommand

	executor := NewExecutor()

	out, err := executor.Run("my_script")

	assert.NoError(t, err)

	expectedOut := fmt.Sprintf("bash -lc shopt -s expand_aliases\nmy_script \"$@\" -- my_script\n")
	assert.Equal(t, expectedOut, out)
}
