package telemetry

import (
	"bytes"
	"os/exec"
	"testing"
)

var mockStatData []byte
var mockMeminfoData []byte

func init() {
	var buf bytes.Buffer
	buf.WriteString("cpu  1000 2000 3000 4000 5000 6000 7000 8000 9000 10000\n")
	for i := 0; i < 128; i++ {
		buf.WriteString("cpu" + string(rune(i)) + " 100 200 300 400 500 600 700 800 900 1000\n")
	}
	buf.WriteString("intr 100 200\nctxt 12345\nbtime 12345\nprocesses 123\nprocs_running 1\nprocs_blocked 0\n")
	mockStatData = buf.Bytes()

	var bufMem bytes.Buffer
	for i := 0; i < 100; i++ {
		bufMem.WriteString("MemTotal:       32655000 kB\n")
		bufMem.WriteString("MemFree:         4567000 kB\n")
		bufMem.WriteString("MemAvailable:   15000000 kB\n")
		bufMem.WriteString("Buffers:          345000 kB\n")
		bufMem.WriteString("Cached:          8000000 kB\n")
		bufMem.WriteString("SwapCached:            0 kB\n")
		bufMem.WriteString("Active:         12000000 kB\n")
		bufMem.WriteString("Inactive:        6000000 kB\n")
		bufMem.WriteString("SwapTotal:       2000000 kB\n")
		bufMem.WriteString("SwapFree:        1500000 kB\n")
	}
	mockMeminfoData = bufMem.Bytes()
}

func BenchmarkGetNetworkInterfaces(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetNetworkInterfaces()
	}
}

func BenchmarkGetSystemUsage(b *testing.B) {
	oldReadFile := readFile
	readFile = func(name string) ([]byte, error) {
		if name == "/proc/stat" {
			return mockStatData, nil
		}
		if name == "/proc/meminfo" {
			return mockMeminfoData, nil
		}
		return nil, nil
	}
	defer func() { readFile = oldReadFile }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetSystemUsage()
	}
}

func BenchmarkGetWifiMode(b *testing.B) {
	// Need to mock execCommand for stable benchmarking
	originalExecCommand := execCommand
	defer func() { execCommand = originalExecCommand }()

	execCommand = func(command string, args ...string) *exec.Cmd {
		// Mock a fast command that returns a simple string to simulate overhead of exec
		// We use true which exits immediately. This still measures the fork/exec overhead.
		return exec.Command("true")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetWifiMode()
	}
}
