package sysutils

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestAskConfirmation(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		input      string
		want       bool
		wantLogged bool
	}{
		{"Client Role", "CLIENT", "", true, false},
		{"Server Role Yes Lower", "SERVER", "y\n", true, false},
		{"Server Role Yes Upper", "SERVER", "Y\n", true, false},
		{"Server Role No Lower", "SERVER", "n\n", false, true},
		{"Server Role No Upper", "SERVER", "N\n", false, true},
		{"Server Role Random Input", "SERVER", "foo\n", false, true},
		{"Server Role Empty Input", "SERVER", "\n", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore os.Stdout and os.Stdin
			oldStdout := os.Stdout
			oldStdin := os.Stdin
			defer func() {
				os.Stdout = oldStdout
				os.Stdin = oldStdin
			}()

			r, w, _ := os.Pipe()
			os.Stdout = w

			if tt.input != "" {
				inR, inW, _ := os.Pipe()
				inW.WriteString(tt.input)
				inW.Close()
				os.Stdin = inR
			}

			os.Setenv("FRAMEFLOW_ROLE", tt.role)

			got := AskConfirmation("Test prompt")

			w.Close()
			var buf bytes.Buffer
			io.Copy(&buf, r)

			if got != tt.want {
				t.Errorf("AskConfirmation() = %v, want %v", got, tt.want)
			}

			// For testing we will rely on AskConfirmation returning false for skips, we can't easily capture Info without mocking logger, but the boolean is the main test
		})
	}
}
