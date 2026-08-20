# VLX FrameFlow

### High-Availability Streaming & Telemetry Suite for SBCs

VLX FrameFlow is a modular Go-based suite designed to transform Debian-based **Single Board Computers (SBCs)** and VPS into:

- High-availability bonding routers (Client & Server)
- Multi-camera streaming encoders
- Real-time telemetry trackers

---

## Architecture

*For comprehensive architecture documentation, please refer to [ARCHITECTURE.md](ARCHITECTURE.md).*

The suite operates via a main installation entry point compiled into a Go binary, internal Go packages, and runtime service managers. Process control is handled via **`systemd`**.

### Execution Privilege & Systemd Architecture

The system enforces a strict operational dichotomy based on the deployed role:

- **CLIENT (SBC/Field Unit):** Strictly operates as an unprivileged dedicated user (`frameflow_user`). Systemd management utilizes User-Space Systemd (`systemctl --user`), stores unit files in `~/.config/systemd/user/`, and explicitly targets `default.target`. Systemd Lingering must be enabled by root during installation.
- **SERVER (VPS/Relay Node):** Strictly requires `root` execution (UID 0) to manage routing and firewall rules. Systemd management utilizes System-Space Systemd (`/etc/systemd/system/`) and targets `multi-user.target`. It relies on Privilege Dropping (`User=`, `Group=`) inside the unit templates to secure exposed services like MediaMTX.

The suite uses a **multi-protocol bonding architecture**: UDP traffic is routed exclusively via MLVPN, and TCP traffic is aggregated using MPTCP with Shadowsocks-libev and v2ray-plugin acting as a transparent proxy.

### Core Structure

The project has evolved into a multi-binary ecosystem to ensure role-specific functionality and minimal footprints:

| Component | Description |
| :--- | :--- |
| **`VLX_FrameFlow`** | Client binary exclusively for SBC tasks (FFmpeg, AP, Storage, API). |
| **`VLX_FrameFlow_SRV`** | Server binary exclusively for VPS tasks (Relay node, Firewall). |
| **`vlx_frontend`** | Remote-capable standalone UI server (Svelte SPA). |
| **`config/`** | Configuration templates and maintenance scripts. |
| **`internal/`** | Core logic (storage, system, network, package management). |

### API & Frontend
The backend API is built using the highly performant **Gin** framework (`github.com/gin-gonic/gin`) serving a standardized REST structure (`/api/...`). Note that status endpoints support both `POST` and `GET` requests.

The **Control Panel Frontend** is a compiled **Svelte** application. It dynamically polls the backend via parallel REST calls (`Promise.all`), features a semantic parser translating raw systemd statuses into visual states, and provides real-time streaming text consoles utilizing `ansi-to-html`.

---

## Installation

We strictly enforce a **"Build as User, Run as Root"** workflow.

### Prerequisites

- A fresh **Debian-based OS** (e.g., Armbian, Debian on VPS).

### Step 1: Clone and Build (As Normal User)

Clone the repository into a recommended local directory (e.g., `~/Project/VLX_FrameFlow`) and build the binaries without elevated privileges:

```bash
mkdir -p ~/Project
cd ~/Project
git clone https://github.com/viruslox/vlx_frameflow.git
cd vlx_frameflow
./build.sh
```

### Step 2: System Configuration (As Root)

Execute the specific role binary as root to begin system configuration. Regardless of whether you install the Client or Server role, the generated target configuration file in `/opt/VLX_FrameFlow/etc/` is universally named `frameflow.settings`.

**For SBC / Field Unit (Client):**
```bash
./build/VLX_FrameFlow_amd64
# Or for ARM devices:
# ./build/VLX_FrameFlow_arm64
```

**For VPS / Relay Node (Server):**
```bash
./build/VLX_FrameFlow_SRV_amd64
# Or for ARM devices:
# ./build/VLX_FrameFlow_SRV_arm64
```

---

## Management

### Interactive Menu

Running the application without arguments provides an interactive menu. The primary choice allows selecting the installation role or updating the suite:

1. **CLIENT (SBC/Field Unit)** - `VLX_FrameFlow` binary provides an automated, heavy installation with its exclusive menu.
2. **SERVER (VPS/Relay Node)** - `VLX_FrameFlow_SRV` binary provides an interactive, conservative installation with its exclusive menu.

#### CLIENT Menu
- **Install OS on Storage:** Clone running OS to high-speed storage.
- **Configure System:** Full setup (FFmpeg, MPTCP, GUI bloatware removal, user config).
- **Reconfigure System network:** Re-apply network features and firewall.
- **Update network interfaces:** Regenerate networkd and hostapd profiles.
- **Install and configure Client components:** MLVPN and Shadowsocks.
- **Complete Clean Up / Roll back:** Reverts client configurations.
- **Exit**.

#### SERVER Menu
- **Install and configure Server components:** MLVPN and Shadowsocks setup with firewall prompts.
- **Complete Clean Up / Roll back:** Safely uninstalls server components and UFW rules.
- **Exit**.

---

## Runtime Tools

All runtime modules are intended to be executed by the **dedicated service user** (default: `frameflow`). They rely on `systemd-run --user` for lifecycle and process management.

### API Command Relay

The **Server** acts as a transparent reverse proxy, relaying local API requests to the remote **Client** (SBC) over the secure MLVPN tunnel (`10.1.10.x`). This enables companion applications (e.g., a Twitch ChatBridge) running on the Server to control the Client securely.

**Flow:**
*   `Twitch Chat` -> `ChatBridge Webhook` -> `FrameFlow Server Relay (<bind_address>)` -> `MLVPN Tunnel` -> `FrameFlow Client API (<relay_client_host>)`

**Example:**
Send an HTTP POST to the local Server to start the Client's cameraman:
```bash
curl -X POST http://127.0.0.1:9090/api/v1/relay/cameraman/start \
  -H "Content-Type: application/json" \
  -d '{"device": "V0A1"}'
```
The Server seamlessly forwards this request (headers and body) through the MLVPN tunnel, where the Client natively executes the command and returns the JSON response back to your local application.

**Example 2:**
Send a GET request to check the Client's MediaMTX status:
```bash
curl http://127.0.0.1:9090/api/v1/relay/mediamtx/status
```

*Note: The FrameFlow relay binds to `<bind_address>:<bind_port>` (defaulting to 127.0.0.1:9090) and serves HTTPS if certificates are configured. If your external webhook (e.g., ChatBridge) is targeting `http://127.0.0.1:8080/api/...`, it will result in a 404 error because port `8080` is strictly bound to the frontend UI.*

---

### Network Flow Control

These commands can be run via the main CLI to switch modes and control networking.

```bash
./vlx_frameflow <command>
```

Available commands:

- **`server start` / `server status` / `server stop`**: Manage server components.
- **`server api start` / `server api status` / `server api stop`**: Manage the local API relay for forwarding commands to the remote Client via MLVPN.
- **`client start` / `client status` / `client stop`**: Manage client components.
- **`client reset`**: Restarts networking and bonding services. (Also exposed via `/api/frameflow/client/reset`).
- **`bonding`**: Displays MPTCP proxy and MLVPN tunnel status.
- **`AP start`**: Activates HostAP (hotspot) on the first Wi-Fi interface.
- **`AP stop`**: Stops HostAP and switches the first Wi-Fi interface back to managed client mode.
- **`AP status`**: Checks if the wifi interface status is coherent with configuration, if not tries to recover.

### Cameraman

```bash
./vlx_frameflow cameraman <VxAy> <start|stop|status>
```

Manages video encoding pipelines. This service is best utilized in combination with direct hardware interfaces such as USB webcams, native camera modules, or HDMI-in capture cards.

- Captures video and audio dynamically via `v4l2-ctl` and `arecord` (bypassing static settings files).
- Streams via **FFmpeg** using SRT protocol (redirects UDP to `10.1.10.1` via MLVPN on Client).

**Parameters:**

- `VxAy`: Video index (`x`) and audio index (`y`). E.g., `V0A1`, or `V1A0` for no audio.

### MediaMTX

```bash
./vlx_frameflow mediamtx <start|stop|status>
```

Manages the local **MediaMTX** server (SRT protocol only). Combining this service with the suite's Access Point (AP) mode is crucial for achieving optimal performance and reliability when interfacing with devices running rigid or proprietary software, such as GoPro cameras. Also exposed via REST (`/api/mediamtx/start|stop|status`).

**Note:** MediaMTX natively operates as a static systemd user service (`frameflow-mediamtx.service` with `Restart=always`). It directly executes the MediaMTX binary using the static configuration file located at `$VLXsuite_DIR/etc/mediamtx.settings`, deprecating older dynamic `systemd-run` and temporary `.yml` generation strategies.

**Functions:**

- **Client Role:** Configured as a lightweight "Push" node. Automatically pushes local camera streams over the MLVPN tunnel (`10.1.10.x`) directly to the Server.
- **Server Role:** Configured as a "Smart Receiver" with WebRTC enabled for Director's Monitoring.

### GPS Tracker

```bash
./vlx_frameflow gps <start|stop|status>
```

Manages GPS and telemetry services. This module acts as a resilient data pipeline, ensuring real-time location and telemetry data are consistently processed and transmitted regardless of network fluctuations. Also exposed via REST (`/api/gps/start|stop|status`).

- Controls **gpsd**.
- Auto-detects USB / serial GPS hardware.
- Reads TPV data via `gpspipe`.
- Pushes JSON telemetry to the configured API endpoint.

---

## Configuration

### `/opt/VLX_FrameFlow/etc/frameflow.settings`

This file contains **all runtime environment variables**. It is generated during the user setup process. Users can safely update this configuration by editing the file with a standard text editor (e.g., `nano /opt/VLX_FrameFlow/etc/frameflow.settings`). Since the services dynamically source this file at runtime, simply restarting the relevant service (such as `VLX_cameraman` or `VLX_gps_tracker`) is sufficient to apply any changes.

*Note: Settings-based hardware mapping for video/audio devices has been deprecated in favor of dynamic CLI arguments (`VxAy`).*


### Apache Reverse Proxy

To serve the Control Panel Frontend behind an Apache Reverse Proxy (for example, under the `/vlx/` path), ensure that the `proxy`, `proxy_http`, and `proxy_wstunnel` modules are enabled. Use the following configuration block:

```apache
<Location /vlx/>
    ProxyPass http://127.0.0.1:8080/
    ProxyPassReverse http://127.0.0.1:8080/
</Location>

<Location /vlx/ws>
    ProxyPass ws://127.0.0.1:8080/ws
    ProxyPassReverse ws://127.0.0.1:8080/ws
</Location>
```
Note: Adjust the port (`8080` by default) to match your `vlx_frontend` bind port.
### General Paths

| Variable | Description |
| :--- | :--- |
| `VLXsuite_DIR` | Installation directory (default: `/opt/VLX_FrameFlow`). |
| `MEDIAMTX_DIR` | MediaMTX binary directory. |

### Network & Security

| Variable | Description |
| :--- | :--- |
| `FRAMEFLOW_USER` | The dedicated, unprivileged user that runs the background services (default: `frameflow`). |
| `MLVPN_SERVER_IP` | The remote server IP for MLVPN (UDP traffic). |
| `MLVPN_SLOT` | MLVPN tunnel identity (client). Must match this client's peer slot on the server. |
| `MLVPN_CLIENT_TUN_IP` | Optional override for client TUN IP. |
| `MLVPN_SERVER_TUN_IP` | Optional override for gateway/server TUN IP. |
| `MLVPN_REMOTE_PORT` | Optional override for remote UDP port. |
| `SHADOWSOCKS_SERVER_IPS` | The remote server IP(s) for Shadowsocks bonding (TCP proxy traffic). |
| `AP_PASSWORD` | WPA2 passphrase for the generated Access Point. Automatically generated if left empty. |
| `MPTCP_PROXY_PASS` | Password for the MPTCP proxy. |
| `MLVPN_KEY` | Key for the single-client MLVPN tunnel. |
| `DB_DSN` | SQLite database DSN for cameraman configuration (default: `/opt/VLX_FrameFlow/var/frameflow.db`). |
| `CAM_PATH_PREFIX` | Remote MediaMTX path prefix (default: `cameraman`). |
| `CAM_MAX_RESOLUTION` | Maximum fallback video resolution for Cameraman (default: `1920x1080`). |
| `CAM_MAX_FPS` | Maximum fallback video FPS for Cameraman (default: `30`). |
| `bind_address` | Backend API bind address (default: `127.0.0.1`). |
| `bind_port` | Backend API bind port (default: `9090`). |
| `allowed_origins` | Comma-separated list of allowed CORS origins for the API. |
| `bkend_user1` / `bkend_pass1` | First tier of backend API credentials. |
| `backend_address` | Target backend address for the Svelte frontend (default: `127.0.0.1`). |
| `backend_port` | Target backend port for the Svelte frontend (default: `9090`). |
| `FF_GUI_USER` / `FF_GUI_PASS` | Credentials used to authenticate to the Svelte Control Panel frontend. |
| `client_crt` / `client_key` | Optional zero-trust mTLS paths for Svelte Control Panel to authenticate against the backend. |
| `relay_client_host` | The remote Client API IP accessed via MLVPN tunnel (default: `10.1.10.2`). |
| `relay_client_port` | The remote Client API Port accessed via MLVPN tunnel (default: `9090`). |
| `use_relay` | For Frontend settings: `true` routes all UI module commands through the relay (`api/v1/relay/...`), useful when hosting UI on Server. `false` targets local Client API. |

**Note on Dual-Stack Bonding:** `SHADOWSOCKS_SERVER_IPS` natively supports Dual-Stack environments. You can provide comma-separated values (e.g., `"34.65.xx.xx, 2001:db8::1"`) to establish concurrent IPv4 and IPv6 tunnels simultaneously.

**Note on Server Configuration:** The Server role configuration (`frameflow_srv.settings.template`) intentionally omits these IP variables. The Server is a destination node that securely binds to `0.0.0.0` and `::` locally, and therefore only requires the corresponding authentication keys (`MLVPN_KEY`, `MPTCP_PROXY_PASS`).

### Streaming Endpoints

Both endpoints undergo **Strict URL Validation** before they are actively bound to the ingest pipeline. A malformed URL configuration (e.g., missing scheme) will proactively prevent the FFmpeg streaming unit from attempting to start, and instead will yield a direct descriptive error in the log.

| Variable | Description |
| :--- | :--- |
| `SRT_URL` | Base SRT URL, explicitly reserved for **Bonded Ingest** over the MLVPN tunnel. Safely auto-parses `publish:` StreamID credentials (`user:pass`). |

**Example:**
```text
srt://10.1.10.1:8890?streamid=publish:stream_name:user:pass
```

### Telemetry (GPS)

| Variable | Description |
| :--- | :--- |
| `GPSPORT` | gpsd local port (default: `1198`). |
| `gps_target_url` | Remote HTTP/HTTPS telemetry endpoint. |

---

## Testing

The repository includes a suite of native Go unit tests.

To run the tests:

```bash
# Run the full test suite
go test ./...
```

Tests cover critical functionality such as system utility functions, configuration parsing, and error handling.

---

## License

**GNU General Public License v3.0**

### Automated Environment Configuration
During the initial installation process as root, the suite automatically registers its commands to the system path:
1. All relevant executables are copied to `/opt/VLX_FrameFlow/bin/`
2. Configuration files are centralized in `/opt/VLX_FrameFlow/etc/`
3. A system-wide environment snippet is created in `/etc/profile.d/vlx_frameflow.sh`

This setup enables standard users and automated services to natively call core suite commands, such as `VLX_FrameFlow status` or `VLX_FrameFlow_SRV config`, from anywhere in the file system without requiring directory navigation.
