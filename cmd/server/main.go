package main

import (
	"fmt"
	"os"

	"github.com/viruslox/vlx_frameflow/cmd/server/cmd"
	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

var (
	Version = "dev"
	Commit  = "unknown"
)

func main() {
	if err := sysutils.CheckPermissions(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Try to install the binary to /usr/local/bin if not already there

	sysutils.Info("Starting VLX_FrameFlow version %s (commit %s)", Version, Commit)
	cmd.Execute()
}
