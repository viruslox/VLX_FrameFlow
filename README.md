# VLX FrameFlow:
# All-in-One Video Streaming and GPS Tracking Suite

****VLX FrameFlow****  is a modular suite of shell scripts designed to transform any Debian-based single-board computer (SBC)
into a high-availability bonding router, a multi-camera video streaming encoder, and a real-time GPS tracking device.

Built for mobility and stability, and security, creating an environment for IRL streaming and fleet tracking.

## Features

-   ****Multi Network Bonding:**** Automatically detects and configures
      multiple internet connections (like 4G/5G modems) to increase total
      bandwitch and provide seamless fault tolerance.
-   ****Multi-Camera Support:**** Manage and stream from multiple V4L2
      video devices simultaneously, including webcams and HDMI capture cards
-   ****Streaming:**** Utilizes ffmpeg to encode and stream video and
      audio to any RTSP or RTMP server (including Twitch and YouTube)
-   ****GPS Tracking:**** Automatically detects a connected GPS device
      and sends location, speed, and altitude to a specified API endpoint.
-   ****Simplified Setup:**** Includes scripts to install the operating
      system on a high-speed NVMe or eMMc drives.

## Use Cases & Applications

****VLXframeflow**** is designed for anyone who needs reliable video and data
transmission from a mobile environment.
****Be Dynamic**** thanks Multi-Camera Content to switch between.
****Stay Stable**** combining multiple internet connections (requires 4G/5G modems).
****Location-Aware**** built-in GPS tracking to create location data overlays.

## For the Commuting Professional
Stay productive and connected when traveling. You can attend meetings
while on a train or in a vehicle trip.

## For IRL Streamers
Take your live streams to the PRO level. With VLXframeflow on an SBC, You can
connect your cameras in a pre-assembled and compact, wearable streaming rig.

## For Transportation and Fleet Management
Equip your fleet with a monitoring solution. Track the precise location of every
vehicle through the integrated GPS tracker, sending data to your own API endpoint.

## Enhanced Phisical Security and Compliance
Use the multi-camera system as a sophisticated dashcam setup to record all angles,
providing evidence in case of issues, events, encounters.

## Architecture Overview
The suite is organized into a modular structure:

- VLX_FrameFlow.sh: The main entry point. An interactive menu for installing the OS, configuring the system, setting up users, and managing updates.

- modules/: Contains the core logic for Storage, System, Network, and Package management.

- config/: Configuration files.

## Runtime Tools:

- VLX_cameraman.sh: Manages video encoding and streaming.

- VLX_gps_tracker.sh: Handles GPS data acquisition and transmission.

- VLX_netflow.sh: Switches between network profiles.

## Getting Started
### 1. Prerequisite
Start with a fresh Debian-based OS (e.g., Raspberry Pi OS Lite 64-bit or Armbian) on an SD card.

### 2. Download & Run the Suite
Clone the repository and launch the main script. You will need root privileges for the initial setup.

```bash
git clone https://github.com/viruslox/VLX_FrameFlow.git
cd VLXframeflow
./VLX_FrameFlow.sh
```

### 3. Interactive Setup Menu
The script will present a menu to guide you through the installation:

- Install OS on Storage: 
	Optional) Clones the running OS from SD card to a high-speed NVMe/SSD drive. Warning: Wipes the target drive.

- Configure System (Full Setup):
	Updates the OS and installs dependencies (FFmpeg, GPSD, MPTCP, etc.).
	Removes desktop bloatware for a headless, optimized performance.

- Security Setup:
	Creates a dedicated user (default: frameflow) and configures sudoers to allow limited privileged actions without a password.

- Update Network Interfaces:
	Generates systemd-networkd profiles for all detected Wi-Fi and LTE/Ethernet interfaces.

- Create/Reconfigure User: 
	Manages the service user permissions.

### 4. Final Configuration
After installation, log in as the dedicated user (e.g., frameflow) created during step 2. Edit your local profile to set your streaming keys and API endpoints:

```bash
nano ~/.frameflow_profile
RTSP_URL / SRT_URL: Set your destination server.
ENABLED_DEVICES: Number of cameras to use.
API_URL: Endpoint for GPS data.
```

## Usage
Note: Always run these tools as the dedicated user, NOT as root.

### Streaming
Start or stop the video stream. You can specify the camera index and protocol.

```bash
./VLX_cameraman.sh 1 start srt
./VLX_cameraman.sh 1 stop
```

### GPS Tracking
Start the GPS daemon and data sender.

```bash
./VLX_gps_tracker.sh start
```

### Network Management
Switch between network profiles (e.g., Bonding mode or standard Wi-Fi client). This script automatically handles permissions via sudo.

```bash
sudo ./VLX_netflow.sh normal
# or
sudo ./VLX_netflow.sh ap-bonding
```

## Maintenance
The suite includes an automated maintenance script (config/FrameFlow_maintenance.sh) configured via cron to:

- Clean up old logs (older than 15 days).
- Backup the list of installed packages.

## Manually update the suite code:

Run sudo ./VLX_FrameFlow.sh

-> Select Option 2 (Configure System)
-> The script automatically detects and pulls the latest changes from GitHub while preserving user permissions.

## License
This project is licensed under the GNU General Public License v3.0. See the LICENSE file for details.
