package sysutils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// AskConfirmation prompts the user for confirmation if FRAMEFLOW_ROLE is SERVER.
// Returns true if confirmed or if role is not SERVER, false otherwise.
func AskConfirmation(prompt string) bool {
	role := os.Getenv("FRAMEFLOW_ROLE")
	if role == "SERVER" {
		fmt.Printf("%s (y/N) ", prompt)
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			Info("Skipping...")
			return false
		}
		input = strings.TrimSpace(input)
		if input == "y" || input == "Y" {
			return true
		}
		Info("Skipping...")
		return false
	}
	return true
}

// AskInput prompts the user for string input.
func AskInput(prompt string) string {
	fmt.Printf("%s: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(input)
}
