# Migration Plan: VLX_FrameFlow from Bash to Golang

This document tracks the incremental migration of the VLX_FrameFlow project from Bash scripts to a native Go application. 
The strategy is to proceed in very small, manageable steps, replacing and testing one module at a time.

## Phase 0: Build Automation (Prerequisite)
Establish a solid build pipeline before writing the application code, mirroring the approach used in `VLX_FrameFlow_GUI`.

- [x] Create an autobuild script (e.g., `build.sh` or `Makefile`) to standardize local compilation.
- [x] Configure cross-compilation targets (e.g., `GOOS=linux GOARCH=arm64`) within the build script to easily generate binaries for single-board computers like the ROCK 5T.
- [x] Implement versioning injection (`-ldflags`) to automatically tag binaries with the current Git commit/tag.
- [x] Set up basic CI workflow (e.g., GitHub Actions) to verify builds on every push.

## Phase 1: Go Infrastructure & Core Setup
Establish the Go environment without breaking the existing Bash execution flow.

- [x] Initialize the Go module (`go mod init github.com/viruslox/vlx_frameflow`).
- [x] Create the standard Go project directory structure (`cmd/frameflow/`, `internal/config/`, `internal/sysutils/`).
- [x] **Config Parser:** Implement a Go struct and parser for `config/mediamtx.settings.template` (using `gopkg.in/yaml.v3`).
- [x] **Logger:** Implement a centralized logging system to eventually replace the Bash `echo` statements (info, warning, error, success).
- [x] **Command Wrapper:** Create an internal helper package around `os/exec` to safely execute external system commands with timeout and error handling.

## Phase 2: System Utilities (`FrameFlow_system.sh` / `FrameFlow_packages.sh`)
Start converting base-level system operations that have no external dependencies.

- [x] **Permissions:** Implement `root`/`sudo` execution checks (`test_check_permissions.sh`).
- [x] **Users:** Port the logic for creating and managing service/sudo users (`test_setup_service_user.sh`).
- [x] **Boot/Aliases:** Implement boot configuration and bash alias generation functions.
- [x] **Packages:** Create a Go wrapper to handle `apt`/`apt-get` package installations and removals safely.
- [x] *Testing:* Write Go unit tests (`sysutils_test.go`) for these utility functions.

## Phase 3: Storage Management (`FrameFlow_storage.sh`)
Replace critical disk operations with Go functions.

- [x] Implement storage device discovery and listing (`test_list_storage_devices.sh`).
- [x] Port GPT partitioning logic (`test_partition_drive_gpt.sh`).
- [x] Implement partition formatting functions (`test_format_partitions.sh`).
- [x] Port the logic for preparing mounts (`test_prepare_mounts.sh`) and note `/etc/fstab` is handled in boot.go.
- [x] *Testing:* Write mock-based tests for storage operations to ensure safety before running on real hardware.

## Phase 4: Networking & Bonding (`FrameFlow_network*.sh` / `FrameFlow_bonding.sh`)
Migrate network configurations incrementally by connection type.

- [x] **Base:** Implement DNS configuration logic (`test_dns_configuration.sh`).
- [x] **Wireless:** Port Wi-Fi interface detection (`test_get_first_wifi_interface.sh`).
- [x] **Wireless:** Implement Access Point and Hostapd configuration generation (`test_create_hostapd_config.sh`).
- [x] **Wired:** Port wired network profile creation.
- [x] **Firewall:** Convert iptables/nftables setup logic (`test_configure_firewall.sh`).
- [x] **Bonding:** Implement MLVPN config and systemd service generation (`test_generate_mlvpn_config.sh`, `test_generate_mlvpn_service.sh`).
- [x] **Bonding:** Port MPTCP kernel checks and configuration (`test_check_mptcp_kernel.sh`).
- [x] *Testing:* Validate that Go-generated configuration files perfectly match the legacy Bash output.

## Phase 5: Services & Daemons (MediaMTX, Cameraman, GPS)
Convert standalone operational scripts into independent Go modules or goroutines.

- [x] **MediaMTX:** Port installation, start, stop, and status logic (`test_mediamtx_*.sh`).
- [x] **Cameraman (Hardware):** Implement video and audio device detection (`test_get_video_device.sh`, `test_get_audio_device.sh`).
- [x] **Cameraman (Stream):** Port stream URL preparation and camera ID parsing (`test_parse_camera_id.sh`).
- [x] **Cameraman (Execution):** Replace `VLX_cameraman.sh` by launching and monitoring `ffmpeg` processes directly via Go.
- [x] **GPS Tracker:** Convert `VLX_gps_tracker.sh` into a Go background worker reading from the serial/gpsd interface.

## Phase 6: CLI Interface (`VLX_FrameFlow.sh`)
Build the new user interface to connect all the Go modules.

- [x] Integrate a CLI framework (e.g., `github.com/spf13/cobra`).
- [x] Replicate the interactive main menu (consider using `github.com/charmbracelet/bubbletea` or `survey` for terminal UI).
- [x] Port user confirmation prompts (`test_ask_confirmation.sh`).
- [x] Wire CLI commands to the underlying Go modules completed in Phases 2-5.

## Phase 7: Final Switch & Cleanup
Ensure feature parity and deprecate the old Bash scripts.

- [x] Compile the final standalone binary via the build script.
- [x] Perform end-to-end testing (Run System Setup, Rollback, etc.) comparing Go binary behavior against the legacy Bash script.
- [x] Convert `benchmark.sh` into native Go benchmarks (`go test -bench`).
- [x] Update `README.md` documentation to reflect Go usage and installation instructions.
- [x] Delete all legacy `.sh` files.

## Migration Plan: Merging GUI Backend into VLX_FrameFlow Core

**Source Repository for Integration:** `https://github.com/viruslox/VLX_FrameFlow_GUI`

This plan details the integration of the backend logic from the GUI repository into the main `VLX_FrameFlow` binary. The Web/Mobile Frontend remains a standalone application, communicating with the core via a secure mTLS channel.

### Phase 8: Preparation & Scaffolding
- [x] Create a new feature branch: `feature/api-backend`.
- [x] Add dependencies to `go.mod`: `gin-gonic/gin`, `gin-contrib/cors`, `gorilla/websocket`.
- [x] Initialize new directory structures:
  - `internal/api/`: REST handlers and WebSocket hub logic.
  - `internal/telemetry/`: Background workers for system/network/GPS metrics.
  - `internal/security/`: CA management and TLS configurations.

### Phase 9: Backend Porting & Native Refactoring
*Objective: Replace shell-script execution (`os/exec`) with native Go function calls.*
- [x] **API Migration:** Port handlers from `VLX_FrameFlow_GUI/vlx_gui_backend/internal/api/` to `internal/api/`.
- [x] **Telemetry Migration:** Port workers from `vlx_gui_backend/internal/system/` to `internal/telemetry/`.
- [x] **Logic Refactoring:** Update API endpoints to invoke `internal/cameraman`, `internal/network`, and `internal/services` packages directly instead of wrapping bash scripts.
- [x] **Error Handling:** Standardize internal errors for consistent JSON API responses.

### Phase 10: Security & Cryptography (mTLS Implementation)
*Goal: Establish a Zero-Trust communication link between the SBC and the remote Frontend.*
- [x] **Local CA Authority:** Implement a mechanism in `internal/security/` to generate a unique local Certificate Authority (CA) on the first run.
- [x] **Backend (Server) Security:**
  - Generate a Server Certificate and Private Key signed by the local CA.
  - Configure the Gin server to enforce mTLS (`tls.RequireAndVerifyClientCert`), validating against the local CA.
- [x] **Client Provisioning:** Create a CLI command (e.g., `vlx_frameflow gui add-client`) to generate and export signed Client Certificates for authorized Frontends/Mobile apps.

### Phase 11: CLI Command Integration
- [x] Create `cmd/frameflow/cmd/api.go` to manage the API lifecycle.
- [x] Implement `vlx_frameflow api start`:
  - [x] Load TLS certificates and initialize the WSHub.
  - [x] Launch telemetry background workers.
  - [x] Start the secure HTTPS server with mTLS and CORS protection.

### Phase 12: Standalone Frontend Updates (Remote Management)
- [x] **Client Authentication:** Update the Frontend server to use the provisioned Client Certificate/Key when connecting to the `VLX_FrameFlow` API.
- [x] **Browser-to-Frontend HTTPS:**
  - **Public Mode:** Integrate `golang.org/x/crypto/acme/autocert` for Let's Encrypt support if a public domain is configured.
  - **Local/Field Mode:** Fallback to a self-signed certificate for local IP access (notifying the user of browser security warnings).
- [x] **Cleanup:** Once the integration is verified, remove the legacy `vlx_gui_backend` folder from the GUI repository.

### Phase 13: Standalone Frontend Integration & Build Automation
*Objective: Migrate the frontend application into the main repository while preserving its strict independence. The frontend must be compiled as a separate, standalone executable that embeds its own web assets and can be deployed anywhere.*

- [x] **Directory Scaffolding & Asset Migration:**
  - Create a new directory `frontend_app/` at the root of the repository to house the Svelte SPA.
  - Copy the entire contents of the external `VLX_FrameFlow_GUI/vlx_gui_frontend/` into `frontend_app/`.
  - Create `cmd/frontend/` to house the standalone Go web server. Copy `vlx_gui_backend/cmd/frontend/main.go` into it.
  - Create `internal/ui/` and copy `vlx_gui_backend/ui/ui.go` into it. 
- [x] **Go Code Refactoring & Import Path Updates:**
  - In `cmd/frontend/main.go`, update the import paths to use the new local module (`github.com/viruslox/vlx_frameflow/internal/ui` and `github.com/viruslox/vlx_frameflow/internal/config`).
  - In `internal/ui/ui.go`, adjust the `//go:embed` directive so that it correctly points to the Svelte build output relative to its new location (e.g., `//go:embed ../../frontend_app/dist/*`).
- [x] **Frontend Application Build Step (`build.sh`):**
  - Modify `build.sh` to include a Node.js build phase: before compiling the Go frontend, the script must navigate into `frontend_app/`, run `npm install`, and then execute `npm run build` to generate the `dist/` folder.
- [x] **Go Standalone Binary Compilation (`build.sh`):**
  - Add a dedicated compilation command in `build.sh` for the frontend executable. It must produce a distinct binary (e.g., `build/vlx_frontend_amd64` and `build/vlx_frontend_arm64`) using `go build -o ... ./cmd/frontend/main.go`.
- [x] **Configuration Isolation Validation:**
  - Ensure the new standalone frontend logic properly loads its configuration (`frontend.settings` or environment variables) completely independently from the main `VLX_FrameFlow` core, preserving the mTLS/AutoCert logic implemented in Phase 12.
- [x] **Documentation Updates:**
  - Update `README.md` to reflect the new dual-binary architecture. Explain that `vlx_frameflow` acts as the core/backend, while `vlx_frontend` is the remote-capable, standalone UI server. Include basic execution instructions for the frontend.
  
### Phase 14: Total Separation of Client and Server Binaries
*Objective: Split the current monolithic architecture into two distinct standalone binaries: `VLX_FrameFlow` (exclusively for SBC/Client tasks) and `VLX_FrameFlow_SRV` (exclusively for VPS/Server tasks). This ensures role-specific menus, reduced binary size, and enhanced security.*

- [x] **Entry Point & Root Command Refactoring:**
  - Create `cmd/client/` to house the Client-only entry point. Migrate relevant commands (Client setup, AP, Cameraman, GPS, API Backend).
  - Create `cmd/server/` to house the Server-only entry point. Migrate relevant commands (Server setup, Server bonding status).
  - Retire the old `cmd/frameflow/` directory once all logic has been redistributed.
- [x] **Menu Exclusivity Implementation:**
  - Refactor `internal/sysutils/ui.go` (or the interactive menu logic) to provide exclusive menus based on the binary being executed.
  - The Client binary menu must only show SBC-related options (FFmpeg, Storage, GPS, etc.).
  - The Server binary menu must only show VPS-related options (Relay node setup, Server firewall).
- [x] **Build Automation Update (`build.sh`):**
  - Update `build.sh` to compile three distinct targets: `VLX_FrameFlow` (Client), `VLX_FrameFlow_SRV` (Server), and `vlx_frontend` (UI Server).
  - Ensure binaries are outputted to the `build/` directory with clear naming conventions for architecture (e.g., `VLX_FrameFlow_arm64`, `VLX_FrameFlow_SRV_amd64`).

### Phase 15: Professional Deployment & Non-Root Workflow
*Objective: Implement a "Build as User, Run as Root" workflow. Users should clone and compile the suite in their local directory without elevated privileges, using root only for final system configuration and service execution.*

- [x] **Non-Root Build Validation:**
  - Audit `build.sh` and the Svelte build process to ensure zero reliance on `sudo` or `root` permissions during the compilation phase.
  - Standardize the recommended deployment directory in documentation (e.g., `~/Project/VLX_FrameFlow`).
- [x] **Permission Enforcement in Binaries:**
  - Implement a mandatory check at the beginning of `VLX_FrameFlow` and `VLX_FrameFlow_SRV` to verify root/sudo privileges.
  - Provide clear, user-friendly error messages if a system-altering command is run by a non-root user.
- [x] **Installation Path Standardization:**
  - Refactor the "Setup" logic so that, upon first run as root, the binaries handle their own "installation" (e.g., copying themselves to `/opt/VLX_FrameFlow/bin/` or `/opt/VLX_FrameFlow/`).
- [x] **Comprehensive Documentation Overhaul:**
  - Rewrite `README.md` to reflect the new workflow:
    - Step 1: Clone and Build in `~/Project` as a normal user.
    - Step 2: Execute the specific binary (`client` or `server`) as root to begin system configuration.
  - Update the "Architecture" section to describe the new multi-binary ecosystem.
  
### Phase 16: Standardized Installation & Directory Hierarchy
*Objective: Standardize the deployment directory structure under `/opt/VLX_FrameFlow/` following Linux FHS best practices for third-party software. The installer will automate the placement of all binaries and configurations, and export the binary path to the user's environment.*

- [x] **Directory Structure Definition:**
  - Define the global base installation path as `/opt/VLX_FrameFlow`.
  - Designate `/opt/VLX_FrameFlow/bin/` as the destination for all executables (`VLX_FrameFlow`, `VLX_FrameFlow_SRV`, `vlx_frontend`).
  - Designate `/opt/VLX_FrameFlow/etc/` as the centralized destination for all configuration and settings files.
- [x] **Configuration File Renaming & Conflict Prevention:**
  - Audit all configuration files to ensure globally unique naming within the shared `etc/` directory (e.g., ensure the core config, frontend settings, and server configs do not overwrite each other).
  - Update the source code in `internal/config/` and `cmd/frontend/main.go` to explicitly search for their respective configuration files in `/opt/VLX_FrameFlow/etc/` by default.
- [x] **Automated Client Installer Update:**
  - Modify the interactive setup logic within the `VLX_FrameFlow` (Client) installer to automatically create the `/opt/VLX_FrameFlow/{bin,etc}` hierarchy.
  - Program the installer to copy both the compiled Client binary (`VLX_FrameFlow`) and the compiled Frontend binary (`vlx_frontend`) into the new `bin/` folder.
  - Program the installer to copy or generate the default configuration templates into the `etc/` folder.
- [x] **Environment PATH Injection:**
  - Add an automated step in the setup routine to inject `/opt/VLX_FrameFlow/bin` into the system's or service user's `$PATH`.
  - Implement this cleanly by creating a profile snippet (e.g., `/etc/profile.d/vlx_frameflow.sh`) or by appending the export statement to the dedicated service user's `~/.bashrc`.
- [x] **Documentation Update:**
  - Document the new `/opt/` directory tree in the `README.md`.
  - Provide instructions demonstrating that users can now run commands natively (e.g., simply typing `VLX_FrameFlow status` instead of `./VLX_FrameFlow status`) thanks to the automated PATH configuration.
