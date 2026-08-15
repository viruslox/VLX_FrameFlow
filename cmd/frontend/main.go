package main

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/viruslox/vlx_frameflow/internal/config"
	"github.com/viruslox/vlx_frameflow/internal/ui"
)

func main() {
	// Load configuration
	customPath := ""
	if len(os.Args) > 1 {
		customPath = os.Args[1]
	}
	cfg := config.LoadConfig(customPath)

	r := gin.Default()

	// Health check endpoint (unauthenticated)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Runtime mode flag for the served UI (unauthenticated: it is not a secret,
	// just tells the SPA whether to call the API directly or via the relay).
	r.GET("/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"use_relay": cfg.UseRelay,
		})
	})

	// Setup BasicAuth
	auth := gin.BasicAuth(gin.Accounts{
		cfg.AuthUser: cfg.AuthPass,
	})

	// Setup Reverse Proxy Target
	targetURL, err := url.Parse(fmt.Sprintf("https://%s:%s", cfg.BackendAddr, cfg.BackendPort))
	if err != nil {
		log.Fatalf("Failed to parse backend URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	if cfg.ClientCrt != "" && cfg.ClientKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ClientCrt, cfg.ClientKey)
		if err != nil {
			log.Fatalf("Failed to load client certificate: %v", err)
		}
		proxy.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true,
			},
		}
	} else {
		proxy.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	// Add Basic Auth for the backend requests
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", cfg.BackendUser, cfg.BackendPass)))

	// Create proxy handler
	proxyHandler := func(c *gin.Context) {
		c.Request.Header.Set("Authorization", authHeader)
		proxy.ServeHTTP(c.Writer, c.Request)
	}

	// Reverse Proxy route for WebSocket (unauthenticated on frontend, validated on backend via ticket)
	r.GET("/ws", proxyHandler)

	// Apply authentication middleware for all other routes
	r.Use(auth)

	// Reverse Proxy routes for API
	r.Any("/api/*filepath", proxyHandler)

	// Serve embedded frontend
	ui.ServeFrontend(r)

	bindAddr := fmt.Sprintf("%s:%s", cfg.BindAddress, cfg.BindPort)
	log.Printf("Starting frontend server on %s", bindAddr)
	if err := r.Run(bindAddr); err != nil {
		log.Fatalf("Frontend server failed to start: %v", err)
	}
}
