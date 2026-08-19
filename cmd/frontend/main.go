package main

import (
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/viruslox/vlx_frameflow/internal/config"
	"github.com/viruslox/vlx_frameflow/internal/network"
	"github.com/viruslox/vlx_frameflow/internal/ui"
)

// This frontend is installed alongside VLX_FrameFlow_SRV and sits behind an
// Apache reverse proxy that terminates browser TLS. Each remote FrameFlow
// client is addressed by its coherent name: the browser calls
// https://<server>/<client-name>/..., and this binary owns all name routing.
//
// The embedded SPA uses a relative Vite base, so serving it under
// /<client-name>/ automatically prefixes every call it makes -- config, API,
// ticket, assets, and the WebSocket URL. This binary strips the name, forwards
// API/WS to the Server relay addressed by that name (/api/v1/peer/<name>/... and
// /ws/<name>), and serves the SPA for everything else. A name is mandatory:
// there is no unnamed root frontend.

func main() {
	customPath := ""
	if len(os.Args) > 1 {
		customPath = os.Args[1]
	}
	cfg := config.LoadConfig(customPath)

	r := gin.Default()

	// Health check (unauthenticated, not client-scoped).
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Reverse proxy to the Server relay, reused for both API and WS.
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
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	backendAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", cfg.BackendUser, cfg.BackendPass)))

	d := &dispatcher{cfg: cfg, proxy: proxy, backendAuth: backendAuth}

	// Only /health is an explicit route; all client-scoped traffic flows through
	// the dispatcher via NoRoute. This sidesteps gin's inability to host a
	// top-level :client param beside static routes.
	r.NoRoute(d.handle)

	bindAddr := fmt.Sprintf("%s:%s", cfg.BindAddress, cfg.BindPort)
	log.Printf("Starting frontend server on %s", bindAddr)
	if err := r.Run(bindAddr); err != nil {
		log.Fatalf("Frontend server failed to start: %v", err)
	}
}

type dispatcher struct {
	cfg         *config.FrontendConfig
	proxy       *httputil.ReverseProxy
	backendAuth string
}

func (d *dispatcher) handle(c *gin.Context) {
	name, rest := splitClient(c.Request.URL.Path)

	// A client name is mandatory; nothing is served at the unnamed root.
	if name == "" {
		c.String(http.StatusNotFound, "VLX FrameFlow: address a client, e.g. /<client-name>/")
		return
	}

	// The name must be a configured client. Validating here lets us reject
	// unknown clients cleanly instead of serving a control panel that would
	// then 404 on every API call.
	if !isKnownClient(name) {
		c.String(http.StatusNotFound, "Unknown client")
		return
	}

	// Trailing-slash redirect: the relative-base SPA only resolves its
	// sub-paths correctly when served from /<name>/ (with the slash).
	if rest == "" {
		target := "/" + name + "/"
		if q := c.Request.URL.RawQuery; q != "" {
			target += "?" + q
		}
		c.Redirect(http.StatusMovedPermanently, target)
		return
	}

	switch {
	case rest == "/config":
		// Client-scoped access is inherently relay mode (the client is remote),
		// so advertise relay regardless of local config.
		c.JSON(http.StatusOK, gin.H{"use_relay": true})

	case rest == "/ws":
		// WebSocket: unauthenticated at the frontend (the ticket is validated on
		// the SBC); forwarded to the Server relay addressed by name.
		c.Request.Header.Set("Authorization", d.backendAuth)
		c.Request.URL.Path = "/ws/" + name
		d.proxy.ServeHTTP(c.Writer, c.Request)

	case strings.HasPrefix(rest, "/api/"):
		if !d.requireBrowserAuth(c) {
			return
		}
		c.Request.Header.Set("Authorization", d.backendAuth)
		c.Request.URL.Path = rewriteAPIByName(rest, name)
		d.proxy.ServeHTTP(c.Writer, c.Request)

	default:
		if !d.requireBrowserAuth(c) {
			return
		}
		ui.ServeSPA(c, rest)
	}
}

// requireBrowserAuth enforces the frontend BasicAuth for browser-facing routes.
func (d *dispatcher) requireBrowserAuth(c *gin.Context) bool {
	u, p, ok := c.Request.BasicAuth()
	userOK := subtle.ConstantTimeCompare([]byte(u), []byte(d.cfg.AuthUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(p), []byte(d.cfg.AuthPass)) == 1
	if !ok || !userOK || !passOK {
		c.Header("WWW-Authenticate", `Basic realm="VLX FrameFlow"`)
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}
	return true
}

// splitClient splits the leading path segment (the client name) from the rest.
// The rest keeps its leading slash; an exact "/<name>" yields rest == "".
func splitClient(p string) (name, rest string) {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return "", ""
	}
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i], p[i:]
	}
	return p, ""
}

// rewriteAPIByName maps the SPA's relay-mode calls onto the Server relay's
// name-addressed peer route: /api/v1/relay/... -> /api/v1/peer/<name>/...
// Anything already peer-addressed or otherwise shaped is forwarded unchanged.
func rewriteAPIByName(rest, name string) string {
	const relayPrefix = "/api/v1/relay"
	if rest == relayPrefix || strings.HasPrefix(rest, relayPrefix+"/") {
		return "/api/v1/peer/" + name + strings.TrimPrefix(rest, relayPrefix)
	}
	return rest
}

// isKnownClient reports whether name matches a configured peer. It reads the
// registry per call so adding a client needs no frontend restart; the registry
// is small and frontend traffic is low.
func isKnownClient(name string) bool {
	peers, err := network.LoadPeers(network.ServerPeersPath())
	if err != nil {
		return false
	}
	_, ok := network.FindPeerByName(peers, name)
	return ok
}
