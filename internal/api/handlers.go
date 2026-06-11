package api

import (
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/viruslox/vlx_frameflow/internal/cameraman"
	"github.com/viruslox/vlx_frameflow/internal/config"
	"github.com/viruslox/vlx_frameflow/internal/network"
	"github.com/viruslox/vlx_frameflow/internal/services/gps"
	"github.com/viruslox/vlx_frameflow/internal/services/mediamtx"
	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

var deviceRegex = regexp.MustCompile(`^V\d+A\d+$`)

type API struct {
	ticketManager *TicketManager
	relayClient   *http.Client
}

func NewAPI(tm *TicketManager) *API {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &API{
		ticketManager: tm,
		relayClient:   &http.Client{Transport: tr},
	}
}

func (a *API) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api")

	// WebSocket Auth Ticket
	api.GET("/ws/ticket", a.handleWSTicket)

	// FrameFlow Client
	api.POST("/frameflow/client/:action", a.handleFrameFlowClient)

	// FrameFlow AP
	api.POST("/frameflow/ap/:action", a.handleFrameFlowAP)

	// FrameFlow Bonding
	api.GET("/frameflow/bonding", a.handleFrameFlowBonding)

	// MediaMTX
	api.POST("/mediamtx/:action", a.handleMediaMTX)

	// GPS Tracker
	api.POST("/gps/:action", a.handleGPS)

	// Cameraman
	api.POST("/cameraman/:action", a.handleCameraman)

	// Client extra actions
	api.POST("/v1/client/reset", HandleClientReset)

	// Client
	api.POST("/v1/client/start", HandleClientStart)
	api.POST("/v1/client/stop", HandleClientStop)
	api.GET("/v1/client/status", HandleClientStatus)

	// AP
	api.POST("/v1/ap/start", HandleAPStart)
	api.POST("/v1/ap/stop", HandleAPStop)
	api.GET("/v1/ap/status", HandleAPStatus)

	// Bonding
	api.POST("/v1/bonding/start", HandleBondingStart)
	api.POST("/v1/bonding/stop", HandleBondingStop)
	api.GET("/v1/bonding/status", HandleBondingStatus)

	// Stream (Cameraman)
	api.POST("/v1/stream/start", HandleStreamStart)
	api.POST("/v1/stream/stop", HandleStreamStop)
	api.GET("/v1/stream/status", HandleStreamStatus)

	// GPS actions
	api.POST("/v1/gps/start", HandleGPSStart)
	api.POST("/v1/gps/stop", HandleGPSStop)
	api.GET("/v1/gps/status", HandleGPSStatus)

	// MediaMTX actions
	api.POST("/v1/mediamtx/start", HandleMediaMTXStart)
	api.POST("/v1/mediamtx/stop", HandleMediaMTXStop)
	api.GET("/v1/mediamtx/status", HandleMediaMTXStatus)

	// Server Relay
	api.Any("/v1/relay/*path", a.handleRelay)

}

func (a *API) handleWSTicket(c *gin.Context) {
	ticket, err := a.ticketManager.GenerateTicket()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate ticket"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ticket": ticket})
}

func (a *API) handleFrameFlowClient(c *gin.Context) {
	action := c.Param("action")
	var err error
	var out string
	switch action {
	case "start":
		err = network.ClientStart()
	case "stop":
		err = network.ClientStop()
	case "status":
		out, err = network.ClientStatus()
	case "reset":
		err = network.ClientReset()
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if out == "" {
		out = "client " + action
	}
	c.JSON(http.StatusOK, gin.H{"output": out})
}

func (a *API) handleFrameFlowAP(c *gin.Context) {
	action := c.Param("action")
	var err error
	switch action {
	case "start":
		err = network.AccesspointStart()
	case "stop":
		err = network.AccesspointStop()
	case "status":
		err = network.AccesspointStatus()
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"output": "AP " + action})
}

func (a *API) handleFrameFlowBonding(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"output": network.GetBondingStatus()})
}

func (a *API) handleMediaMTX(c *gin.Context) {
	action := c.Param("action")
	var err error
	var out string
	switch action {
	case "start", "stop", "status":
		exePath, exeErr := os.Executable()
		if exeErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get executable path: " + exeErr.Error()})
			return
		}
		exePath, exeErr = filepath.EvalSymlinks(exePath)
		if exeErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve executable path: " + exeErr.Error()})
			return
		}
		out, err = sysutils.RunCommand(10*time.Second, exePath, "mediamtx", action)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if out == "" {
		out = "mediamtx " + action
	}
	c.JSON(http.StatusOK, gin.H{"output": out})
}

func (a *API) handleGPS(c *gin.Context) {
	action := c.Param("action")
	var err error
	var out string
	switch action {
	case "start":
		// We'll just pass a dummy port, or maybe we don't need it.
		// For now let's just do it
		err = gps.StartGPSD("/dev/ttyUSB0")
	case "stop":
		err = gps.StopGPSD()
	case "status":
		out = gps.StatusGPSD()
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if out == "" {
		out = "gps " + action
	}
	c.JSON(http.StatusOK, gin.H{"output": out})
}

type CameramanRequest struct {
	Device string `json:"device"` // e.g. V0A1
}

func (a *API) handleCameraman(c *gin.Context) {
	action := c.Param("action")
	if action != "start" && action != "stop" && action != "status" && action != "list-dev" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
		return
	}

	var req CameramanRequest
	if c.Request.ContentLength > 0 && c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
	}

	// If device is not in JSON, try query param for compatibility
	if req.Device == "" {
		req.Device = c.Query("device")
	}

	if action == "list-dev" {
		out, err := cameraman.ListDevices()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"output": out})
		return
	}

	if req.Device != "" && !deviceRegex.MatchString(req.Device) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device parameter format"})
		return
	}

	if action == "status" {
		if req.Device == "" {
			out, err := cameraman.StatusAllStreams()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"output": out})
			return
		}
		out, err := cameraman.StatusStream(req.Device)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"output": out})
		return
	}

	if req.Device == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device parameter is required for this action"})
		return
	}

	var err error
	if action == "start" {
		// we'll just parse the device
		vidID, audID, err := cameraman.ParseCameraID(req.Device)
		if err == nil {
			// dummy URL, mode, ffmpegPath for compilation
			err = cameraman.StartStream(req.Device, vidID, audID)
		}
	} else if action == "stop" {
		err = cameraman.StopStream(req.Device)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"output": "cameraman " + action})
}

func (a *API) handleRelay(c *gin.Context) {
	// Retrieve port from config or use 9090 as default
	backendCfg := config.LoadBackendConfig("")
	clientPort := backendCfg.BindPort
	if clientPort == "" {
		clientPort = "9090"
	}

	path := c.Param("path")

	// Construct the target HTTPS URL
	targetURL := "https://10.1.10.2:" + clientPort + "/api" + path
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	// Construct a new http.Request
	req, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
		return
	}

	// Copy headers
	for name, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// Forward the request
	resp, err := a.relayClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to relay request: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(name, value)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(c.Writer, resp.Body)
}

// --- Client Reset ---
func HandleClientReset(c *gin.Context) {
	if err := network.ClientReset(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "client reset initiated"})
}

// --- GPS Handlers ---
func HandleGPSStart(c *gin.Context) {
	if err := gps.StartGPSD("/dev/ttyUSB0"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "gps started"})
}

func HandleGPSStop(c *gin.Context) {
	if err := gps.StopGPSD(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "gps stopped"})
}

func HandleGPSStatus(c *gin.Context) {
	status := gps.StatusGPSD()
	c.JSON(http.StatusOK, gin.H{"status": status})
}

// --- MediaMTX Handlers ---
func HandleMediaMTXStart(c *gin.Context) {
	if err := mediamtx.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "mediamtx started"})
}

func HandleMediaMTXStop(c *gin.Context) {
	if err := mediamtx.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "mediamtx stopped"})
}

func HandleMediaMTXStatus(c *gin.Context) {
	status := mediamtx.Status()
	c.JSON(http.StatusOK, gin.H{"status": status})
}

// --- Client Handlers ---
func HandleClientStart(c *gin.Context) {
	if err := network.ClientStart(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "client started"})
}

func HandleClientStop(c *gin.Context) {
	if err := network.ClientStop(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "client stopped"})
}

func HandleClientStatus(c *gin.Context) {
	status, err := network.ClientStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

// --- AP Handlers ---
func HandleAPStart(c *gin.Context) {
	if err := network.SystemAccesspointStart(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ap started"})
}

func HandleAPStop(c *gin.Context) {
	if err := network.SystemAccesspointStop(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ap stopped"})
}

func HandleAPStatus(c *gin.Context) {
	if err := network.SystemAccesspointStatus(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ap is active"})
}

// --- Bonding Handlers ---
func HandleBondingStart(c *gin.Context) {
	if err := network.SetupMlvpnBonding(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "MLVPN setup failed: " + err.Error()})
		return
	}
	// Also attempt to set up mptcp proxy if part of bonding start sequence (based on system scripts context)
	network.SetupMptcpProxy()
	c.JSON(http.StatusOK, gin.H{"status": "bonding started"})
}

func HandleBondingStop(c *gin.Context) {
	role := os.Getenv("FRAMEFLOW_ROLE")
	if role == "SERVER" {
		sysutils.RunCommand(10*time.Second, "systemctl", "stop", "frameflow-mlvpn.service")
		sysutils.RunCommand(10*time.Second, "systemctl", "stop", "frameflow-mptcp-proxy.service")
	} else {
		sysutils.RunCommand(10*time.Second, "systemctl", "--user", "stop", "frameflow-mlvpn.service")
		sysutils.RunCommand(10*time.Second, "systemctl", "--user", "stop", "frameflow-mptcp-proxy.service")
	}
	c.JSON(http.StatusOK, gin.H{"status": "bonding stopped"})
}

func HandleBondingStatus(c *gin.Context) {
	status := network.GetBondingStatus()
	c.JSON(http.StatusOK, gin.H{"status": status})
}

// --- Stream (Cameraman) Handlers ---
func HandleStreamStart(c *gin.Context) {
	var req CameramanRequest
	if c.Request.ContentLength > 0 && c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
	}
	if req.Device == "" {
		req.Device = c.Query("device")
	}

	if req.Device == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device parameter is required"})
		return
	}

	if !deviceRegex.MatchString(req.Device) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device parameter format"})
		return
	}

	vidID, audID, err := cameraman.ParseCameraID(req.Device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := cameraman.StartStream(req.Device, vidID, audID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stream started"})
}

func HandleStreamStop(c *gin.Context) {
	var req CameramanRequest
	if c.Request.ContentLength > 0 && c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
	}
	if req.Device == "" {
		req.Device = c.Query("device")
	}

	if req.Device == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device parameter is required"})
		return
	}

	if err := cameraman.StopStream(req.Device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stream stopped"})
}

func HandleStreamStatus(c *gin.Context) {
	device := c.Query("device")
	if device == "" {
		status, err := cameraman.StatusAllStreams()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": status})
		return
	}

	status, err := cameraman.StatusStream(device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}
