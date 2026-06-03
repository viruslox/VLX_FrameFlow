package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/viruslox/vlx_frameflow/internal/sysutils"
	"time"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server related commands",
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start server components",
	Run: func(cmd *cobra.Command, args []string) {
		sysutils.Info("Starting server components...")
		var hasError bool

		if _, err := sysutils.RunCommand(10*time.Second, "systemctl", "start", "frameflow-mptcp-proxy.service"); err != nil {
			sysutils.Error("Failed to start frameflow-mptcp-proxy.service: %v", err)
			hasError = true
		}

		if _, err := sysutils.RunCommand(10*time.Second, "systemctl", "start", "frameflow-mlvpn.service"); err != nil {
			sysutils.Error("Failed to start frameflow-mlvpn.service: %v", err)
			hasError = true
		}

		if !hasError {
			sysutils.Success("Server components started.")
		} else {
			sysutils.Error("Some server components failed to start.")
		}
	},
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check server components status",
	Run: func(cmd *cobra.Command, args []string) {
		sysutils.Info("Checking server components status...")
		if out, _ := sysutils.RunCommand(10*time.Second, "systemctl", "status", "frameflow-mptcp-proxy.service", "--no-pager"); out != "" {
			fmt.Println(out)
		}
		if out, _ := sysutils.RunCommand(10*time.Second, "systemctl", "status", "frameflow-mlvpn.service", "--no-pager"); out != "" {
			fmt.Println(out)
		}
	},
}

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop server components",
	Run: func(cmd *cobra.Command, args []string) {
		sysutils.Info("Stopping server components...")
		sysutils.RunCommand(10*time.Second, "systemctl", "stop", "frameflow-mptcp-proxy.service")
		sysutils.RunCommand(10*time.Second, "systemctl", "stop", "frameflow-mlvpn.service")
		sysutils.Info("Killing possible orphan processes...")
		sysutils.RunCommand(10*time.Second, "pkill", "-f", "ss-server")
		sysutils.RunCommand(10*time.Second, "pkill", "-f", "ss-redir")
		sysutils.RunCommand(10*time.Second, "pkill", "-f", "mlvpn")
		sysutils.RunCommand(10*time.Second, "pkill", "-f", "v2ray-plugin")
		sysutils.Success("Server components stopped and orphans killed.")
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.AddCommand(serverStartCmd, serverStatusCmd, serverStopCmd)
}
