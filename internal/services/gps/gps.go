package gps

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

var DevDir = "/dev/"

func GetGPSDevice() (string, error) {
	matches, err := filepath.Glob(filepath.Join(DevDir, "ttyACM*"))
	if err != nil {
		matches = []string{}
	}
	usbMatches, err := filepath.Glob(filepath.Join(DevDir, "ttyUSB*"))
	if err == nil {
		matches = append(matches, usbMatches...)
	}

	var bestDev string
	var bestTime time.Time

	for _, devPath := range matches {
		info, err := os.Stat(devPath)
		if err != nil {
			continue // Skip if we can't stat it (e.g. not really there)
		}

		// Use udevadm to check if it's a modem/LTE device
		out, err := sysutils.RunCommand(5*time.Second, "udevadm", "info", "-q", "property", "-n", devPath)
		if err == nil {
			lowerOut := strings.ToLower(out)
			if strings.Contains(lowerOut, "modem") ||
			   strings.Contains(lowerOut, "lte") ||
			   strings.Contains(lowerOut, "airprime") ||
			   strings.Contains(lowerOut, "wwan") {
				continue // Skip this device, it's a modem/LTE
			}
		}

		if bestDev == "" || info.ModTime().After(bestTime) {
			bestDev = filepath.Base(devPath)
			bestTime = info.ModTime()
		}
	}

	if bestDev == "" {
		return "", nil // No valid device found
	}

	return bestDev, nil
}

func StartGPSD(gpsPort string) error {
	gpsdUnit := "frameflow-gpsd"

	sysutils.RunCommand(5*time.Second, "systemctl", "--user", "reset-failed", "frameflow-gpsd")
	sysutils.RunCommand(5*time.Second, "systemctl", "--user", "reset-failed", "frameflow-gps-sender")

	// Check if already running
	_, err := sysutils.RunCommand(5*time.Second, "systemctl", "--user", "is-active", "--quiet", gpsdUnit)
	if err == nil {
		sysutils.Info("GPS Tracker services are already running.")
		return nil
	}

	device, err := GetGPSDevice()
	if err != nil || device == "" {
		return fmt.Errorf("No GPS device found (ttyACM/ttyUSB). Aborting.")
	}
	devicePath := "/dev/" + device
	sysutils.Info("Found GPS device: %s", devicePath)

	gpsdBinOut, err := sysutils.RunCommand(5*time.Second, "which", "gpsd")
	if err != nil {
		return fmt.Errorf("gpsd binary not found: %w", err)
	}
	gpsdBin := strings.TrimSpace(gpsdBinOut)

	// Start GPSD Service
	sysutils.Info("Starting GPSD on port %s...", gpsPort)
	_, err = sysutils.RunCommand(10*time.Second, "systemd-run", "--user",
		"--unit="+gpsdUnit,
		"--description=VLX FrameFlow GPS Daemon",
		"--collect",
		"--property=Restart=on-failure",
		"--property=RestartSec=5",
		"--service-type=exec",
		gpsdBin, "-N", "-n", "-S", gpsPort, devicePath)

	if err != nil {
		return fmt.Errorf("Failed to start GPSD: %w", err)
	}

	vlxClientPath, _ := os.Executable()
	_, err = sysutils.RunCommand(10*time.Second, "systemd-run", "--user",
		"--unit=frameflow-gps-sender",
		"--description=VLX FrameFlow GPS Sender",
		"--collect",
		"--property=Restart=on-failure",
		"--property=RestartSec=5",
		"--service-type=exec",
		vlxClientPath, "gps", "sender")
	if err != nil {
		return fmt.Errorf("Failed to start GPS Sender: %w", err)
	}

	return nil
}

func StopGPSD() error {
	sysutils.Info("Stopping services...")
	sysutils.RunCommand(10*time.Second, "systemctl", "--user", "stop", "frameflow-gps-sender")
	sysutils.RunCommand(10*time.Second, "systemctl", "--user", "stop", "frameflow-gpsd")
	return nil
}

func StatusGPSD() (string, error) {
	outD, _ := sysutils.RunCommand(10*time.Second, "systemctl", "--user", "status", "frameflow-gpsd", "--no-pager")
	return fmt.Sprintf("--- GPSD Service ---\n%s", outD), nil
}

type TPVReport struct {
	Class string  `json:"class"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Alt   float64 `json:"altMSL"`
	Epx   float64 `json:"epx"`
	Speed float64 `json:"speed"`
}

type Payload struct {
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Alt      float64 `json:"alt"`
	PosError float64 `json:"pos_error"`
	Speed    float64 `json:"speed"`
}

func RunSender(ctx context.Context, gpsPort, targetURL string) error {
	sysutils.Info("Starting native Go GPS API Sender...")

	address := "localhost:" + gpsPort
	var conn net.Conn
	var err error

	// Retry loop to connect to gpsd
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		conn, err = net.Dial("tcp", address)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	defer conn.Close()

	// Tell gpsd to start sending JSON reports
	_, err = conn.Write([]byte("?WATCH={\"enable\":true,\"json\":true}\n"))
	if err != nil {
		return fmt.Errorf("failed to write watch command to gpsd: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	client := &http.Client{Timeout: 10 * time.Second}

	var lastSend time.Time
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Drain socket at max speed, process only every 5 seconds
		if time.Since(lastSend) < 5*time.Second {
			continue
		}

		line := scanner.Bytes()
		var report TPVReport

		// Fast check for class TPV before full unmarshal
		if !bytes.Contains(line, []byte(`"class":"TPV"`)) {
			continue
		}

		if err := json.Unmarshal(line, &report); err != nil {
			sysutils.Warning("Failed to unmarshal TPV report: %v", err)
			continue
		}

		if report.Class != "TPV" {
			continue
		}

		payload := Payload{
			Lat:      report.Lat,
			Lon:      report.Lon,
			Alt:      report.Alt,
			PosError: report.Epx,
			Speed:    report.Speed,
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			sysutils.Warning("Failed to marshal payload: %v", err)
			continue
		}

		req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewBuffer(jsonData))
		if err != nil {
			sysutils.Warning("Failed to create HTTP request: %v", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		lastSend = time.Now()
		// Fire and forget POST request
		go func(r *http.Request) {
			resp, err := client.Do(r)
			if err != nil {
				sysutils.Warning("Failed to send GPS data: %v", err)
				return
			}
			defer func() {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}()

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				sysutils.Warning("API returned non-200 status code: %d", resp.StatusCode)
			}
		}(req)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading from gpsd: %w", err)
	}

	return nil
}
