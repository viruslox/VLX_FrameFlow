package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

var serverCmd = &cobra.Command{
	Use:   "server <start|stop|status|install|uninstall|enable|disable>",
	Short: "Server related commands",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() != 0 {
			fmt.Println("Error: this command requires root privileges. Run via sudo or as root.")
			os.Exit(1)
		}

		action := args[0]

		switch action {
		case "start":
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
		case "status":
			sysutils.Info("Checking server components status...")
			if out, _ := sysutils.RunCommand(10*time.Second, "systemctl", "status", "frameflow-mptcp-proxy.service", "--no-pager"); out != "" {
				fmt.Println(out)
			}
			if out, _ := sysutils.RunCommand(10*time.Second, "systemctl", "status", "frameflow-mlvpn.service", "--no-pager"); out != "" {
				fmt.Println(out)
			}
		case "stop":
			sysutils.Info("Stopping server components...")
			sysutils.RunCommand(10*time.Second, "systemctl", "stop", "frameflow-mptcp-proxy.service")
			sysutils.RunCommand(10*time.Second, "systemctl", "stop", "frameflow-mlvpn.service")
			sysutils.Info("Killing possible orphan processes...")
			sysutils.RunCommand(10*time.Second, "pkill", "-f", "ss-server")
			sysutils.RunCommand(10*time.Second, "pkill", "-f", "ss-redir")
			sysutils.RunCommand(10*time.Second, "pkill", "-f", "mlvpn")
			sysutils.RunCommand(10*time.Second, "pkill", "-f", "v2ray-plugin")
			sysutils.Success("Server components stopped and orphans killed.")
		case "enable":
			sysutils.RunCommand(10*time.Second, "systemctl", "enable", "frameflow-server.service")
			sysutils.Success("Server daemon enabled at boot.")
		case "disable":
			sysutils.RunCommand(10*time.Second, "systemctl", "disable", "frameflow-server.service")
			sysutils.Success("Server daemon disabled at boot.")
		case "install":
			serviceContent := `[Unit]
Description=VLX FrameFlow Server Daemon
After=network.target

[Service]
Type=simple
ExecStart=/opt/VLX_FrameFlow/bin/VLX_FrameFlow_SRV server start
Restart=on-failure

[Install]
WantedBy=multi-user.target
`
			err := os.WriteFile("/etc/systemd/system/frameflow-server.service", []byte(serviceContent), 0644)
			if err != nil {
				sysutils.Error("Failed to write systemd unit file: %v", err)
				os.Exit(1)
			}
			sysutils.RunCommand(10*time.Second, "systemctl", "daemon-reload")
			sysutils.Success("Server daemon installed.")
		case "uninstall":
			os.Remove("/etc/systemd/system/frameflow-server.service")
			sysutils.RunCommand(10*time.Second, "systemctl", "daemon-reload")
			sysutils.Success("Server daemon uninstalled.")
		default:
			fmt.Printf("Unknown action: %s. Use start, stop, status, install, uninstall, enable, or disable.\n", action)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
