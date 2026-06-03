package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/viruslox/vlx_frameflow/internal/api"
	"github.com/viruslox/vlx_frameflow/internal/config"
	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "API relay commands for Server",
}

var apiStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the local API relay server",
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: api command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}

		sysutils.Info("Starting local API relay server...")

		backendCfg := config.LoadBackendConfig("")

		r := gin.Default()

		tm := api.NewTicketManager()
		apiHandlers := api.NewAPI(tm)
		apiHandlers.RegisterRoutes(r)

		addr := fmt.Sprintf("127.0.0.1:%s", backendCfg.BindPort)
		if backendCfg.BindPort == "" {
			addr = "127.0.0.1:9090"
		}

		sysutils.Info("Starting local HTTP relay server on %s", addr)

		if err := api.StartLocalServer(addr, r); err != nil {
			sysutils.Error("Failed to start API server: %v", err)
		}
	},
}

var apiStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check local API relay server status",
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: api command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}

		sysutils.Info("Checking local API relay server status...")
		out, _ := sysutils.RunCommand(10*time.Second, "pgrep", "-f", "[V]LX_FrameFlow_SRV.*api start")
		if out != "" {
			fmt.Println("API relay server is running")
		} else {
			fmt.Println("API relay server is not running")
		}
	},
}

var apiStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the local API relay server",
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: api command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}

		sysutils.Info("Stopping local API relay server...")
		sysutils.RunCommand(10*time.Second, "pkill", "-f", "[V]LX_FrameFlow_SRV.*api start")
		sysutils.Success("API relay server stopped.")
	},
}

func init() {
	rootCmd.AddCommand(apiCmd)
	apiCmd.AddCommand(apiStartCmd, apiStatusCmd, apiStopCmd)
}
