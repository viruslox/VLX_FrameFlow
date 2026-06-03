package sysutils

import (
	"fmt"
	"os/exec"
	"testing"
)

func BenchmarkSystemctlLoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for j := 1; j <= 50; j++ {
			unit := fmt.Sprintf("dummy%d.service", j)
			cmd := exec.Command("systemctl", "status", unit, "--no-pager")
			_ = cmd.Run()
		}
	}
}

func BenchmarkSystemctlBatched(b *testing.B) {
	units := make([]string, 0, 50)
	for j := 1; j <= 50; j++ {
		units = append(units, fmt.Sprintf("dummy%d.service", j))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		args := append([]string{"status"}, units...)
		args = append(args, "--no-pager")
		cmd := exec.Command("systemctl", args...)
		_ = cmd.Run()
	}
}
