package api

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"

	"github.com/viruslox/vlx_frameflow/internal/network"
)

// ResolvePeerClientHostByID resolves a peer by an identifier that is either a
// numeric slot or a client name, returning the client's in-tunnel API host. A
// numeric id defers to ResolvePeerClientHost (which also honours legacy
// single-client mode); a name requires a peer registry.
func ResolvePeerClientHostByID(id, fallbackHost string) (string, error) {
	if slot, err := strconv.Atoi(id); err == nil {
		return ResolvePeerClientHost(slot, fallbackHost)
	}

	peersPath := network.ServerPeersPath()
	if _, err := os.Stat(peersPath); err != nil {
		return "", fmt.Errorf("client %q requested but no peer registry present", id)
	}
	peers, err := network.LoadPeers(peersPath)
	if err != nil {
		return "", err
	}
	if p, ok := network.FindPeerByName(peers, id); ok {
		return p.ClientTunIP, nil
	}
	return "", fmt.Errorf("no client named %q in registry", id)
}

// ResolvePeerClientHost maps a peer slot to the client's in-tunnel API host.
//
// When a peer registry (etc/peers.yaml) is present it is authoritative: the
// slot must exist there and its (derived or overridden) client tunnel IP is
// returned. Without a registry the server is in legacy single-client mode, so
// only slot 0 is valid and it resolves to the legacy fallback host
// (RelayClientHost / 10.1.10.2).
func ResolvePeerClientHost(slot int, fallbackHost string) (string, error) {
	peersPath := network.ServerPeersPath()
	if _, err := os.Stat(peersPath); err == nil {
		peers, err := network.LoadPeers(peersPath)
		if err != nil {
			return "", err
		}
		for _, p := range peers {
			if p.Slot == slot {
				return p.ClientTunIP, nil
			}
		}
		return "", fmt.Errorf("no peer with slot %d in registry", slot)
	}
	if slot == 0 {
		if fallbackHost == "" {
			fallbackHost = "10.1.10.2"
		}
		return fallbackHost, nil
	}
	return "", fmt.Errorf("peer slot %d requested but no peer registry present", slot)
}

// NewClientWSProxy builds a reverse proxy to a client's WebSocket telemetry hub
// at https://host:port. It strips the browser Origin so the SBC's CheckOrigin
// treats the upgrade as a non-cross-origin (tunnelled) request and accepts it,
// mirroring how the HTTP relay avoids imposing CORS on relayed calls. The
// ticket is validated on the SBC, so no auth logic lives here.
func NewClientWSProxy(host, port string) *httputil.ReverseProxy {
	target := &url.URL{Scheme: "https", Host: fmt.Sprintf("%s:%s", host, port)}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.Header.Del("Origin")
		req.Host = target.Host
	}
	return proxy
}
