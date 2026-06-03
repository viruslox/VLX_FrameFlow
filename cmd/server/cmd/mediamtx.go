package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/viruslox/vlx_frameflow/internal/services/mediamtx"
)

var mediamtxCmd = &cobra.Command{
	Use:   "mediamtx <start|stop|status|install>",
	Short: "Manages the local MediaMTX server",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		action := args[0]

		switch action {
		case "start":
			fmt.Println("Starting MediaMTX server...")
			err := mediamtx.Start()
			if err != nil {
				fmt.Printf("Error starting MediaMTX: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("MediaMTX server started successfully.")

		case "stop":
			fmt.Println("Stopping MediaMTX server...")
			err := mediamtx.Stop()
			if err != nil {
				fmt.Printf("Error stopping MediaMTX: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("MediaMTX server stopped successfully.")

		case "status":
			err := mediamtx.Status()
			if err != nil {
				fmt.Printf("MediaMTX status: %v\n", err)
				os.Exit(1)
			}

		case "install":
			err := mediamtx.Install()
			if err != nil {
				fmt.Printf("MediaMTX install error: %v\n", err)
				os.Exit(1)
			}

		default:
			fmt.Printf("Unknown action: %s. Use start, stop, status, or install.\n", action)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(mediamtxCmd)
}
