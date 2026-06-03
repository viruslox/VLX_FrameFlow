package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install VLX_FrameFlow",
	Long:  `Installs VLX_FrameFlow binaries, configurations and configures the user`,
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() != 0 {
			fmt.Println("Error: install command must be run as root.")
			os.Exit(1)
		}

		err := sysutils.InstallBinary(false)
		if err != nil {
			fmt.Println("Install failed:", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
