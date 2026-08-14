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

		bindAddr := backendCfg.BindAddress
		port := backendCfg.BindPort
		if port == "" {
			port = "9090"
		}
		addr := fmt.Sprintf("%s:%s", bindAddr, port)
		if bindAddr == "" {
			// Preserve prior default: bind loopback only when no address is set.
			addr = fmt.Sprintf("127.0.0.1:%s", port)
		}

		// Resolve cert/key/CA: prefer explicit config, else the standard install
		// location. When a cert+key are present, serve HTTPS (server-auth TLS,
		// client cert optional); otherwise fall back to plain HTTP so existing
		// local-only deployments keep working unchanged.
		certPath := backendCfg.ServerCrt
		keyPath := backendCfg.ServerKey
		var caPath string
		secDir := "/opt/VLX_FrameFlow/certs/"
		if _, err := os.Stat(secDir); os.IsNotExist(err) {
			secDir = "."
		}
		if certPath == "" {
			certPath = filepath.Join(secDir, "server.crt")
		}
		if keyPath == "" {
			keyPath = filepath.Join(secDir, "server.key")
		}
		caPath = filepath.Join(secDir, "ca.crt")
		if _, err := os.Stat(caPath); os.IsNotExist(err) {
			caPath = "" // optional; StartServer tolerates an empty CA path
		}

		_, certErr := os.Stat(certPath)
		_, keyErr := os.Stat(keyPath)
		if certErr == nil && keyErr == nil {
			sysutils.Info("Starting HTTPS relay server on %s", addr)
			if err := api.StartServer(addr, certPath, keyPath, caPath, r); err != nil {
				sysutils.Error("Failed to start HTTPS API server: %v", err)
			}
		} else {
			sysutils.Warning("Server cert/key not found (%s / %s); falling back to plain HTTP. The frontend requires HTTPS -- generate certs to enable it.", certPath, keyPath)
			sysutils.Info("Starting local HTTP relay server on %s", addr)
			if err := api.StartLocalServer(addr, r); err != nil {
				sysutils.Error("Failed to start API server: %v", err)
			}
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
