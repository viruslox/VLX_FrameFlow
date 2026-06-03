package telemetry

import (
	"encoding/json"
	"time"
)

type TelemetryPayload struct {
	Type      string      `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

func StartWorkers(broadcast func([]byte)) {
	// System Usage & Network Stats Worker (Every 1 second)
	go func() {
		for {
			payload := TelemetryPayload{
				Type:      "system_network_stats",
				Timestamp: time.Now().Unix(),
				Data: map[string]interface{}{
					"system_usage":       GetSystemUsage(),
					"network_interfaces": GetNetworkInterfaces(),
					"wifi_mode":          GetWifiMode(),
				},
			}
			data, err := json.Marshal(payload)
			if err == nil {
				broadcast(data)
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// GPS Data Worker (Every 5 seconds)
	go func() {
		for {
			gpsData := GetGPSData()
			if gpsData != "" {
				payload := TelemetryPayload{
					Type:      "gps_data",
					Timestamp: time.Now().Unix(),
					Data:      gpsData,
				}
				data, err := json.Marshal(payload)
				if err == nil {
					broadcast(data)
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()

	// FFmpeg Logs Worker (Every 5 seconds)
	go func() {
		for {
			ffmpegLogs := GetFFmpegLogs()
			if ffmpegLogs != "" {
				payload := TelemetryPayload{
					Type:      "ffmpeg_logs",
					Timestamp: time.Now().Unix(),
					Data:      ffmpegLogs,
				}
				data, err := json.Marshal(payload)
				if err == nil {
					broadcast(data)
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()
}
