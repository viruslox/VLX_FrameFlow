package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"




	"github.com/spf13/cobra"
	"github.com/viruslox/vlx_frameflow/internal/network"
)

var apCmd = &cobra.Command{
	Use:   "ap",
	Aliases: []string{"AP"},
	Short: "Wireless interface commands",
}

var apStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start AP mode on the first wifi interface",
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: AP command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}
		binary, err := os.Executable()
		if err != nil {
			binary = "VLX_FrameFlow"
		}
		out, err := sysutils.RunCommand(30*time.Second, "sudo", binary, "ap", "_ap_system_ops", "start")
		if out != "" {
			fmt.Print(out)
		}
		if err != nil {
			os.Exit(1)
		}
	},
}

var apStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop AP mode on the first wifi interface",
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: AP command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}
		binary, err := os.Executable()
		if err != nil {
			binary = "VLX_FrameFlow"
		}
		out, err := sysutils.RunCommand(30*time.Second, "sudo", binary, "ap", "_ap_system_ops", "stop")
		if out != "" {
			fmt.Print(out)
		}
		if err != nil {
			os.Exit(1)
		}
	},
}

var apStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check is the wifi interface status is coherent with configuration, if not tries to recover.",
	Run: func(cmd *cobra.Command, args []string) {
		status, err := network.AccesspointStatus()
		if err != nil {
			sysutils.Error("Error checking AP status: %v", err)
			os.Exit(1)
		}
		fmt.Println(status)
	},
}



var apSystemOpsCmd = &cobra.Command{
	Use:    "_ap_system_ops [start|stop|status]",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() != 0 {
			fmt.Println("Error: internal system operations must be run as root.")
			os.Exit(1)
		}
		if len(args) == 0 {
			os.Exit(1)
		}
		switch args[0] {
		case "start":
			if err := network.SystemAccesspointStart(); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		case "stop":
			if err := network.SystemAccesspointStop(); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		case "status":
			status := network.SystemAccesspointStatus()
			fmt.Println(status)
		}
	},
}

func init() {
	rootCmd.AddCommand(apCmd)
	apCmd.AddCommand(apStartCmd, apStopCmd, apStatusCmd, apSystemOpsCmd)
}
