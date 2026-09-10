//go:build !linux

package telemetry

func GetNetworkInterfaces() map[string]NetworkInterfaceStats {
	return make(map[string]NetworkInterfaceStats)
}

func GetSystemUsage() SystemUsage {
	return SystemUsage{
		Temps: make(map[string]float64),
	}
}

func GetGPSData() string {
	return ""
}

func GetFFmpegLogs() string {
	return ""
}
