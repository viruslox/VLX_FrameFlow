package telemetry

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"syscall"
)

var scriptNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type CommandExecutor interface {
	Run(scriptName string, args ...string) (string, error)
}

var execCommand = exec.Command

type Executor struct {
}

func NewExecutor() CommandExecutor {
	return &Executor{}
}

// Run executes a bash script by name with the given arguments.
// It relies on the interactive bash shell to expand aliases like VLX_FrameFlow
// setup by the environment.
func (e *Executor) Run(scriptName string, args ...string) (string, error) {
	if !scriptNameRegex.MatchString(scriptName) {
		return "", fmt.Errorf("invalid script name: %s", scriptName)
	}

	// Reconstruct the command to be executed by bash -lc
	// We pass the alias as the command string and arguments as positional
	// parameters to bash to avoid command injection.
	// We need to use "$@" inside the bash script string to forward arguments.
	bashCmdString := fmt.Sprintf("shopt -s expand_aliases\n%s \"$@\"", scriptName)

	// Construct the exec.Cmd: bash -lc 'shopt -s expand_aliases\nalias_name "$@"' -- scriptName arg1 arg2 ...
	cmdArgs := []string{"-lc", bashCmdString, "--", scriptName}
	cmdArgs = append(cmdArgs, args...)

	cmd := execCommand("bash", cmdArgs...)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to execute %s: %v, stderr: %s", scriptName, err, stderr.String())
	}

	return out.String(), nil
}
