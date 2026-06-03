package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "frameflow",
	Short: "VLX FrameFlow Setup",
	Long:  `VLX FrameFlow Setup - A robust migration to Golang`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do Setup Menu logic here if no arguments
		runInteractiveMenu()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
