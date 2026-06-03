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

The suite uses a **multi-protocol bonding architecture**: UDP traffic is routed exclusively via MLVPN, and TCP traffic is aggregated using MPTCP with Shadowsocks-libev and v2ray-plugin acting as a transparent proxy.

### Core Structure

The project has evolved into a multi-binary ecosystem to ensure role-specific functionality and minimal footprints:

| Component | Description |
| :--- | :--- |
| **`VLX_FrameFlow`** | Client binary exclusively for SBC tasks (FFmpeg, AP, Storage, API). |
| **`VLX_FrameFlow_SRV`** | Server binary exclusively for VPS tasks (Relay node, Firewall). |
| **`vlx_frontend`** | Remote-capable standalone UI server. |
| **`config/`** | Configuration templates and maintenance scripts. |
| **`internal/`** | Core logic (storage, system, network, package management). |

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

Execute the specific role binary as root to begin system configuration:

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

### Network Flow Control

These commands can be run via the main CLI to switch modes and control networking.

```bash
./vlx_frameflow <command>
```

Available commands:

- **`server start` / `server status` / `server stop`**: Manage server components.
- **`server api start` / `server api status` / `server api stop`**: Manage the local API relay for forwarding commands to the remote Client via MLVPN.
- **`client start` / `client status` / `client stop`**: Manage client components.
- **`client reset`**: Restarts networking and bonding services.
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

Manages the local **MediaMTX** server (SRT protocol only). Combining this service with the suite's Access Point (AP) mode is crucial for achieving optimal performance and reliability when interfacing with devices running rigid or proprietary software, such as GoPro cameras.

**Functions:**

- Acts as a relay/proxy for local streams.
- Restreams to remote endpoints via SRT.
- Dynamically injects FFmpeg commands into `mediamtx.settings`.

### GPS Tracker

```bash
./vlx_frameflow gps <start|stop|status>
```

Manages GPS and telemetry services. This module acts as a resilient data pipeline, ensuring real-time location and telemetry data are consistently processed and transmitted regardless of network fluctuations.

- Controls **gpsd**.
- Auto-detects USB / serial GPS hardware.
- Reads TPV data via `gpspipe`.
- Pushes JSON telemetry to the configured API endpoint.

---

## Configuration

### `/opt/VLX_FrameFlow/etc/frameflow.settings`

This file contains **all runtime environment variables**. It is generated during the user setup process. Users can safely update this configuration by editing the file with a standard text editor (e.g., `nano /opt/VLX_FrameFlow/etc/frameflow.settings`). Since the services dynamically source this file at runtime, simply restarting the relevant service (such as `VLX_cameraman` or `VLX_gps_tracker`) is sufficient to apply any changes.

*Note: Settings-based hardware mapping for video/audio devices has been deprecated in favor of dynamic CLI arguments (`VxAy`).*

### General Paths

| Variable | Description |
| :--- | :--- |
| `VLXsuite_DIR` | Installation directory (default: `/opt/VLX_FrameFlow`). |
| `MEDIAMTX_DIR` | MediaMTX binary directory. |

### Network & Security

| Variable | Description |
| :--- | :--- |
| `MLVPN_VPS_IP` | The remote server IP for MLVPN and Shadowsocks bonding. |
| `AP_PASSWORD` | WPA2 passphrase for the generated Access Point. Automatically generated if left empty. |

### Streaming Endpoints

| Variable | Description |
| :--- | :--- |
| `SRT_URL` | Base SRT URL. Supports `publish:` StreamID auth. |

**Example:**
```text
srt://10.1.10.1:8890?streamid=publish:stream_name:user:pass
```

### Telemetry (GPS)

| Variable | Description |
| :--- | :--- |
| `GPSPORT` | gpsd local port (default: `1198`). |
| `API_URL` | Remote HTTP/HTTPS telemetry endpoint. |
| `AUTH_TOKEN` | Bearer token for API authentication. |

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
