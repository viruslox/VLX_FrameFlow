package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/viruslox/vlx_frameflow/internal/network"
	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

var bondingCmd = &cobra.Command{
	Use:   "bonding",
	Short: "Check server components status",
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() != 0 {
			fmt.Println("Error: this command requires root privileges. Run via sudo or as root.")
			os.Exit(1)
		}

		os.Setenv("FRAMEFLOW_ROLE", "SERVER")
		sysutils.Info("Bonding Status:")
		fmt.Print(network.GetBondingStatus())
	},
}

func init() {
	rootCmd.AddCommand(bondingCmd)
}
