# VLX FrameFlow API Reference

> **Part of the VLX Stream Flow ecosystem — Edge Acquisition & Transport tier.**

VLX FrameFlow manages the edge SBC operations (cameras, bonding, telemetry) and relays commands through the remote VPS server. In the ecosystem, it receives HTTP requests from **ChatBridge** via the `webhook` transport.

---

## 1. FrameFlow Server Relay

The FrameFlow Server (running on the VPS) exposes a relay API that securely forwards requests over the MLVPN tunnel to the Client (SBC). ChatBridge uses this to trigger commands without needing direct access to the SBC.

**Base Relay URL:** `http://127.0.0.1:9090/api/v1/relay/`

The URL pattern is `/api/v1/relay/<module>/<action>`.

### Modules and Actions

| Module | Action | Purpose |
| :--- | :--- | :--- |
| `cameraman` | `/start` | Starts the camera capture and SRT encoding loop. Accepts JSON payload `{"device":"V0A1"}`. |
| `cameraman` | `/stop` | Stops camera capture. |
| `cameraman` | `/status` | Returns the status of the cameraman service. |
| `cameraman` | `/list-dev` | Returns available V4L2 video devices on the SBC. |
| `mediamtx` | `/start` | Starts the internal MediaMTX instance (wificam RTMP ingest). |
| `mediamtx` | `/stop` | Stops the internal MediaMTX instance. |
| `mediamtx` | `/status` | Returns the MediaMTX status. |
| `gps` | `/start` | Starts the GPS telemetry transmitter (`gpsd` to `ChatBridge`). |
| `gps` | `/stop` | Stops the GPS transmitter. |
| `gps` | `/status` | Returns GPS transmitter status. |
| `frameflow/client` | `/start` | Starts the core FrameFlow client services. |
| `frameflow/client` | `/stop` | Stops the core FrameFlow client services. |
| `frameflow/client` | `/reset` | Soft resets the services. |
| `frameflow/bonding` | `/start` | Enables MLVPN/MPTCP bonded interfaces. |
| `frameflow/bonding` | `/stop` | Disables bonded interfaces. |
| `frameflow/ap` | `/start` | Starts the local Wi-Fi Hotspot on the SBC. |
| `frameflow/ap` | `/stop` | Stops the Wi-Fi Hotspot. |

---

## 2. Examples for ChatBridge

You can trigger any of these endpoints using ChatBridge's `webhook` transport inside a `static/chat/owner_<command>.json` file.

### Example: Start Cameraman (Camera 1)
```json
{
  "description": "Start field unit camera",
  "auto_delete": true,
  "actions": [
    {
      "transport": "webhook",
      "method": "POST",
      "url": "http://127.0.0.1:9090/api/v1/relay/cameraman/start",
      "payload": {
        "device": "V0A1"
      }
    }
  ]
}
```

### Example: Enable Hotspot
```json
{
  "description": "Start SBC Wi-Fi AP",
  "actions": [
    {
      "transport": "webhook",
      "method": "POST",
      "url": "http://127.0.0.1:9090/api/v1/relay/frameflow/ap/start"
    }
  ]
}
```

### Example: Stop GPS Telemetry
```json
{
  "description": "Stop GPS tracking",
  "actions": [
    {
      "transport": "webhook",
      "method": "POST",
      "url": "http://127.0.0.1:9090/api/v1/relay/gps/stop"
    }
  ]
}
```

## 3. GPS Telemetry Egress (SBC to ChatBridge)

While ChatBridge controls FrameFlow via HTTP, FrameFlow constantly pushes telemetry back to ChatBridge.

The SBC `gps` service routinely performs an HTTP POST to ChatBridge (`POST http://10.1.10.1:8000/api/gps`) containing the following payload structure:

```json
{
  "lat": 45.4642,
  "lon": 9.1900,
  "alt": 120.5,
  "pos_error": 5.2,
  "speed": 12.3
}
```
*Note: This data is internally intercepted by ChatBridge, formatted, and emitted over WebSockets for overlays without user configuration needed.*
