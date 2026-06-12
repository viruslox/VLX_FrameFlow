package api

import (
	"crypto/tls"
	"io"
	"net/http"
	"os"
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

var deviceRegex = regexp.MustCompile(`^(V|A)\d+$`)

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

	// Group: /api/frameflow
	frameflow := api.Group("/frameflow")
	{
		// Client
		frameflow.POST("/client/start", HandleClientStart)
		frameflow.POST("/client/stop", HandleClientStop)
		frameflow.POST("/client/status", HandleClientStatus)
		frameflow.POST("/client/reset", HandleClientReset)

		// AP
		frameflow.POST("/ap/start", HandleAPStart)
		frameflow.POST("/ap/stop", HandleAPStop)
		frameflow.POST("/ap/status", HandleAPStatus)

		// Bonding
		frameflow.POST("/bonding/start", HandleBondingStart)
		frameflow.POST("/bonding/stop", HandleBondingStop)
		frameflow.POST("/bonding/status", HandleBondingStatus)
	}

	// Group: /api/cameraman
	cameramanGroup := api.Group("/cameraman")
	{
		cameramanGroup.POST("/start", HandleStreamStart)
		cameramanGroup.POST("/stop", HandleStreamStop)
		cameramanGroup.POST("/status", HandleStreamStatus)
		cameramanGroup.POST("/list-dev", HandleStreamListDev)
	}

	// Group: /api/mediamtx
	mediamtxGroup := api.Group("/mediamtx")
	{
		mediamtxGroup.POST("/start", HandleMediaMTXStart)
		mediamtxGroup.POST("/stop", HandleMediaMTXStop)
		mediamtxGroup.POST("/status", HandleMediaMTXStatus)
	}

	// Group: /api/gps
	gpsGroup := api.Group("/gps")
	{
		gpsGroup.POST("/start", HandleGPSStart)
		gpsGroup.POST("/stop", HandleGPSStop)
		gpsGroup.POST("/status", HandleGPSStatus)
	}

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

type CameramanRequest struct {
	Device string `json:"device"` // e.g. V0A1
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
	status := network.SystemAccesspointStatus()
	c.JSON(http.StatusOK, gin.H{"status": status})
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

	hwType, id, err := cameraman.ParseCameraID(req.Device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := cameraman.StartStream(req.Device, hwType, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "Stream " + req.Device + " started successfully."})
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
	c.JSON(http.StatusOK, gin.H{"status": "Stream " + req.Device + " stopped successfully."})
}

func HandleStreamStatus(c *gin.Context) {
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
		status, err := cameraman.StatusAllStreams()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": status})
		return
	}

	status, err := cameraman.StatusStream(req.Device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

func HandleStreamListDev(c *gin.Context) {
	out, err := cameraman.ListDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"output": out})
}
