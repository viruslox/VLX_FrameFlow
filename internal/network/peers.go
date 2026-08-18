package network

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// MlvpnPeer describes a single bonded client on the SERVER side of the MLVPN
// tunnel plane. Each peer maps to its own mlvpn instance: a dedicated tun
// interface, UDP port, address pair, config file, and templated systemd unit.
//
// The Slot deterministically derives the network resources so a minimal
// peers.yaml only needs slot + name + key:
//
//	interface   mlvpn{slot}
//	udp port    5080 + slot
//	server IP   10.1.{10+slot}.1/24   (this server's tunnel address)
//	client IP   10.1.{10+slot}.2/24   (the client's tunnel address / its gateway)
//
// Slot 0 reproduces the historical single-client values (mlvpn0, 5080,
// 10.1.10.1 / 10.1.10.2), so existing deployments remain byte-compatible.
//
// ClientTunIP, ServerTunIP and Port may be set explicitly to override the
// derived defaults; leave them empty/zero to derive from Slot.
type MlvpnPeer struct {
	Slot        int    `yaml:"slot"`
	Name        string `yaml:"name"`
	Key         string `yaml:"key"`
	ClientTunIP string `yaml:"client_tun_ip"`
	ServerTunIP string `yaml:"server_tun_ip"`
	Port        int    `yaml:"port"`
}

// peersFile is the on-disk shape of etc/peers.yaml.
type peersFile struct {
	Peers []MlvpnPeer `yaml:"peers"`
}

// basePort is the UDP port of slot 0; higher slots increment from here.
const basePort = 5080

// maxSlot bounds the peer slot to the same two-digit NN range used elsewhere
// in the suite (cameraman path slots), and keeps the derived third octet and
// UDP port comfortably in range.
const maxSlot = 99

// reservedPorts are UDP/other ports the server already binds; an explicitly
// overridden peer Port must not collide with these.
var reservedPorts = map[int]struct{}{
	22:   {}, // ssh
	53:   {}, // dns
	67:   {}, // dhcp
	123:  {}, // ntp
	546:  {}, // dhcpv6
	547:  {}, // dhcpv6
	1080: {}, // shadowsocks local
	8189: {}, // mediamtx webrtc
	8322: {}, // mediamtx rtsp
	8388: {}, // shadowsocks server
	8889: {}, // mediamtx webrtc
	8890: {}, // mediamtx srt
}

// ServerPeersPath returns the canonical location of the peer registry,
// resolved against VLXsuite_DIR (default /opt/VLX_FrameFlow).
func ServerPeersPath() string {
	vlxDir := os.Getenv("VLXsuite_DIR")
	if vlxDir == "" {
		vlxDir = "/opt/VLX_FrameFlow"
	}
	return filepath.Join(vlxDir, "etc", "peers.yaml")
}

// PeerInterface returns the tun interface name for a slot (e.g. "mlvpn3").
func PeerInterface(slot int) string {
	return fmt.Sprintf("mlvpn%d", slot)
}

// PeerServiceInstance returns the templated systemd instance name for a slot
// (e.g. "frameflow-mlvpn@3.service").
func PeerServiceInstance(slot int) string {
	return fmt.Sprintf("frameflow-mlvpn@%d.service", slot)
}

// DerivePeerDefaults fills any empty/zero network fields from the slot. It does
// not touch Key or Name.
func DerivePeerDefaults(p *MlvpnPeer) {
	octet := 10 + p.Slot
	if p.ServerTunIP == "" {
		p.ServerTunIP = fmt.Sprintf("10.1.%d.1", octet)
	}
	if p.ClientTunIP == "" {
		p.ClientTunIP = fmt.Sprintf("10.1.%d.2", octet)
	}
	if p.Port == 0 {
		p.Port = basePort + p.Slot
	}
}

// ClientTunnelIdentity is the resolved client-side view of its MLVPN tunnel:
// which addresses to claim and which server bind port to dial. It is the
// mirror of a server peer entry and is derived through the same rules, so a
// client on slot k always agrees with the server's peer slot k.
type ClientTunnelIdentity struct {
	Slot        int
	ClientTunIP string // this client's tun address (e.g. 10.1.10.2)
	ServerTunIP string // the server's tun address / in-tunnel gateway (e.g. 10.1.10.1)
	RemotePort  int    // server bind port to connect to (e.g. 5080)
}

// DeriveClientTunnelIdentity builds a client identity from a slot plus optional
// explicit overrides (empty string / zero means derive from slot). Slot 0 with
// no overrides reproduces the historical 10.1.10.2 / gw 10.1.10.1 / udp 5080.
func DeriveClientTunnelIdentity(slot int, clientTunIP, serverTunIP string, remotePort int) ClientTunnelIdentity {
	p := MlvpnPeer{Slot: slot, ClientTunIP: clientTunIP, ServerTunIP: serverTunIP, Port: remotePort}
	DerivePeerDefaults(&p)
	return ClientTunnelIdentity{
		Slot:        slot,
		ClientTunIP: p.ClientTunIP,
		ServerTunIP: p.ServerTunIP,
		RemotePort:  p.Port,
	}
}

// validatePeers enforces slot bounds and cross-peer uniqueness of the resources
// that would otherwise silently clash (slot/interface, port, address pair).
func validatePeers(peers []MlvpnPeer) error {
	if len(peers) == 0 {
		return nil
	}
	seenSlot := map[int]struct{}{}
	seenPort := map[int]struct{}{}
	seenSrv := map[string]struct{}{}
	seenCli := map[string]struct{}{}

	for _, p := range peers {
		if p.Slot < 0 || p.Slot > maxSlot {
			return fmt.Errorf("peer %q: slot %d out of range 0..%d", p.Name, p.Slot, maxSlot)
		}
		if _, dup := seenSlot[p.Slot]; dup {
			return fmt.Errorf("duplicate peer slot %d", p.Slot)
		}
		seenSlot[p.Slot] = struct{}{}

		if p.Port < 1 || p.Port > 65535 {
			return fmt.Errorf("peer %q (slot %d): port %d out of range", p.Name, p.Slot, p.Port)
		}
		if _, bad := reservedPorts[p.Port]; bad {
			return fmt.Errorf("peer %q (slot %d): port %d is reserved by another server service", p.Name, p.Slot, p.Port)
		}
		if _, dup := seenPort[p.Port]; dup {
			return fmt.Errorf("peer %q (slot %d): duplicate udp port %d", p.Name, p.Slot, p.Port)
		}
		seenPort[p.Port] = struct{}{}

		if p.ServerTunIP == p.ClientTunIP {
			return fmt.Errorf("peer %q (slot %d): server and client tunnel IP are identical (%s)", p.Name, p.Slot, p.ServerTunIP)
		}
		if _, dup := seenSrv[p.ServerTunIP]; dup {
			return fmt.Errorf("peer %q (slot %d): duplicate server tunnel IP %s", p.Name, p.Slot, p.ServerTunIP)
		}
		seenSrv[p.ServerTunIP] = struct{}{}
		if _, dup := seenCli[p.ClientTunIP]; dup {
			return fmt.Errorf("peer %q (slot %d): duplicate client tunnel IP %s", p.Name, p.Slot, p.ClientTunIP)
		}
		seenCli[p.ClientTunIP] = struct{}{}
	}
	return nil
}

// LoadPeers reads and validates the peer registry, deriving per-slot defaults.
// Peers are returned sorted by slot for deterministic ordering. A registry with
// zero peers is valid (it provisions no tunnels) and returns an empty slice.
func LoadPeers(path string) ([]MlvpnPeer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read peers registry %s: %w", path, err)
	}

	var pf peersFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("failed to parse peers registry %s: %w", path, err)
	}

	for i := range pf.Peers {
		DerivePeerDefaults(&pf.Peers[i])
	}
	sort.Slice(pf.Peers, func(i, j int) bool { return pf.Peers[i].Slot < pf.Peers[j].Slot })

	if err := validatePeers(pf.Peers); err != nil {
		return nil, err
	}
	return pf.Peers, nil
}

// generateKey returns a fresh random hex key for a peer that has none.
func generateKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate peer key: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// EnsurePeerKey persists a generated key for the given slot into peers.yaml,
// editing the YAML node tree in place so comments and any other fields are
// preserved. It sets the "key" field only for the matching peer's mapping and
// is a no-op if that peer already carries a non-empty key. Returns the key in
// effect for the slot.
func EnsurePeerKey(path string, slot int, key string) (string, error) {
	if key != "" {
		return key, nil
	}
	newKey, err := generateKey()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read peers registry %s: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("failed to parse peers registry %s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		return "", fmt.Errorf("peers registry %s is empty", path)
	}

	root := doc.Content[0] // top-level mapping
	seq := mappingValue(root, "peers")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return "", fmt.Errorf("peers registry %s has no 'peers' sequence", path)
	}

	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		slotNode := mappingValue(item, "slot")
		if slotNode == nil || slotNode.Value != fmt.Sprintf("%d", slot) {
			continue
		}
		if keyNode := mappingValue(item, "key"); keyNode != nil {
			if keyNode.Value != "" {
				return keyNode.Value, nil // already set on disk; keep it
			}
			keyNode.Value = newKey
			keyNode.Tag = "!!str"
			keyNode.Style = yaml.DoubleQuotedStyle
		} else {
			item.Content = append(item.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "key"},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: newKey, Style: yaml.DoubleQuotedStyle},
			)
		}

		out, err := yaml.Marshal(&doc)
		if err != nil {
			return "", fmt.Errorf("failed to re-marshal peers registry: %w", err)
		}
		if err := os.WriteFile(path, out, 0600); err != nil {
			return "", fmt.Errorf("failed to write peers registry %s: %w", path, err)
		}
		return newKey, nil
	}
	return "", fmt.Errorf("peers registry %s has no peer with slot %d", path, slot)
}

// mappingValue returns the value node for the given key within a mapping node,
// or nil if absent.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}
