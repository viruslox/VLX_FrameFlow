package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/viruslox/vlx_frameflow/internal/services/gps"
)

var gpsCmd = &cobra.Command{
	Use:   "gps <start|stop|status|sender>",
	Short: "Manages GPS and telemetry services",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: gps command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}

		vlxSuiteDir := os.Getenv("VLXsuite_DIR")
		if vlxSuiteDir == "" {
			vlxSuiteDir = "/opt/VLX_FrameFlow"
		}

		settingsPath := filepath.Join(vlxSuiteDir, "etc", "frameflow.settings")
		_ = godotenv.Load(settingsPath)

		action := args[0]

		gpsPort := os.Getenv("GPSPORT")
		if gpsPort == "" {
			gpsPort = "1198"
		}

		switch action {
		case "start":
			fmt.Printf("Starting GPS Tracker on port %s...\n", gpsPort)
			err := gps.StartGPSD(gpsPort)
			if err != nil {
				fmt.Printf("Error starting GPS Tracker: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("GPS Tracker started successfully.")

		case "stop":
			fmt.Println("Stopping GPS Tracker...")
			err := gps.StopGPSD()
			if err != nil {
				fmt.Printf("Error stopping GPS Tracker: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("GPS Tracker stopped successfully.")

		case "status":
			out, err := gps.StatusGPSD()
			if err != nil {
				fmt.Printf("Error checking status for GPS Tracker: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(out)

		case "sender":
			// Hidden command used by the background service to run the sender
			targetURL := os.Getenv("gps_target_url")

			if targetURL == "" {
				fmt.Println("Error: gps_target_url environment variable is not set")
				os.Exit(1)
			}

			err := gps.RunSender(context.Background(), gpsPort, targetURL)
			if err != nil {
				fmt.Printf("Error running GPS sender: %v\n", err)
				os.Exit(1)
			}

		default:
			fmt.Printf("Unknown action: %s. Use start, stop, or status.\n", action)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(gpsCmd)
}
