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
	Short: "Check client components status",
	Run: func(cmd *cobra.Command, args []string) {
		os.Setenv("FRAMEFLOW_ROLE", "CLIENT")
		sysutils.Info("Bonding Status:")
		fmt.Print(network.GetBondingStatus())
	},
}

func init() {
	rootCmd.AddCommand(bondingCmd)
}
