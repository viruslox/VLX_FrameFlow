package cmd

import (
	"fmt"
	"os"
	"github.com/spf13/cobra"
	"github.com/viruslox/vlx_frameflow/internal/network"
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Client related commands",
}

var clientStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start client components",
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: client command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}
		if err := network.ClientStart(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

var clientStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check client components status",
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: client command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}
		out, err := network.ClientStatus()
		if out != "" {
			fmt.Println(out)
		}
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

var clientStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop client components",
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: client command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}
		if err := network.ClientStop(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

var clientResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Restart client networking and bonding services",
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: client command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}
		network.ClientReset()
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)
	clientCmd.AddCommand(clientStartCmd, clientStatusCmd, clientStopCmd, clientResetCmd)
}
