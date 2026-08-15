package cmd

import (
	"os"
	"path/filepath"

	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/viruslox/vlx_frameflow/internal/api"
	"github.com/viruslox/vlx_frameflow/internal/config"
	"github.com/viruslox/vlx_frameflow/internal/security"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"
	"github.com/viruslox/vlx_frameflow/internal/telemetry"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "API related commands",
}

var apiStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the secure HTTPS server",
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: api command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}

		sysutils.Info("Starting API server...")

		// Load configuration
		backendCfg := config.LoadBackendConfig("")

		// Check if running in production or development
		secDir := "/opt/VLX_FrameFlow/certs/"
		if _, err := os.Stat(secDir); os.IsNotExist(err) {
			secDir = "." // Fallback to current dir for dev/testing
		}

		caCertPath := filepath.Join(secDir, "ca.crt")
		caKeyPath := filepath.Join(secDir, "ca.key")
		serverCertPath := backendCfg.ServerCrt
		if serverCertPath == "" {
			serverCertPath = filepath.Join(secDir, "server.crt")
		}
		serverKeyPath := backendCfg.ServerKey
		if serverKeyPath == "" {
			serverKeyPath = filepath.Join(secDir, "server.key")
		}

		if err := security.EnsureLocalCA(caCertPath, caKeyPath); err != nil {
			sysutils.Error("Failed to ensure local CA: %v", err)
			return
		}

		if _, err := os.Stat(serverCertPath); os.IsNotExist(err) {
			caCertPEM, err := os.ReadFile(caCertPath)
			if err != nil {
				sysutils.Error("Failed to read CA certificate: %v", err)
				return
			}
			caKeyPEM, err := os.ReadFile(caKeyPath)
			if err != nil {
				sysutils.Error("Failed to read CA private key: %v", err)
				return
			}
			serverCertPEM, serverKeyPEM, err := security.GenerateServerCert(caCertPEM, caKeyPEM)
			if err != nil {
				sysutils.Error("Failed to generate server certificate: %v", err)
				return
			}
			if err := os.WriteFile(serverCertPath, serverCertPEM, 0644); err != nil {
				sysutils.Error("Failed to write server certificate: %v", err)
				return
			}
			if err := os.WriteFile(serverKeyPath, serverKeyPEM, 0600); err != nil {
				sysutils.Error("Failed to write server private key: %v", err)
				return
			}
		}

		sysutils.Info("TLS certificates loaded successfully")

		// Load TLS certificates and initialize the WSHub
		wsHub := api.NewWSHub([]string{"*"})
		go wsHub.Run()
		telemetry.StartWorkers(wsHub.Broadcast)

		r := gin.Default()


		// Setup CORS middleware
		corsConfig := cors.DefaultConfig()
		if len(backendCfg.AllowedOrigins) > 0 {
			corsConfig.AllowOrigins = backendCfg.AllowedOrigins
			corsConfig.AllowAllOrigins = false
		} else {
			corsConfig.AllowAllOrigins = true
		}
		corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
		corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
		corsConfig.ExposeHeaders = []string{"Content-Length"}
		// When AllowAllOrigins is true, AllowCredentials must not be true or Gin panics.
		if corsConfig.AllowAllOrigins {
			corsConfig.AllowCredentials = false
		} else {
			corsConfig.AllowCredentials = true
		}
		r.Use(cors.New(corsConfig))

		tm := api.NewTicketManager()
		apiHandlers := api.NewAPI(tm)
		apiHandlers.RegisterRoutes(r, false) // CLIENT: full local routes

		r.GET("/ws", func(c *gin.Context) {
			wsHub.HandleWebSocket(c, tm)
		})

		addr := fmt.Sprintf("%s:%s", backendCfg.BindAddress, backendCfg.BindPort)
		if backendCfg.BindAddress == "" {
			addr = fmt.Sprintf(":%s", backendCfg.BindPort)
		}
		sysutils.Info("Starting HTTPS server on %s", addr)

		if err := api.StartServer(addr, serverCertPath, serverKeyPath, caCertPath, r); err != nil {
			sysutils.Error("Failed to start API server: %v", err)
		}
	},
}

var apiStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check API server status",
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: api command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}

		sysutils.Info("Checking API server status...")
		out, _ := sysutils.RunCommand(10*time.Second, "pgrep", "-f", "[V]LX_FrameFlow.*api start")
		if out != "" {
			fmt.Println("API server is running")
		} else {
			fmt.Println("API server is not running")
		}
	},
}

var apiStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the API server",
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: api command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}

		sysutils.Info("Stopping API server...")
		sysutils.RunCommand(10*time.Second, "pkill", "-f", "[V]LX_FrameFlow.*api start")
		sysutils.Success("API server stopped.")
	},
}

func init() {
	rootCmd.AddCommand(apiCmd)
	apiCmd.AddCommand(apiStartCmd, apiStatusCmd, apiStopCmd)
}
