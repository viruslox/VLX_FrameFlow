package network

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeriveClientTunnelIdentity_Slot0Legacy(t *testing.T) {
	id := DeriveClientTunnelIdentity(0, "", "", 0)
	if id.ClientTunIP != "10.1.10.2" || id.ServerTunIP != "10.1.10.1" || id.RemotePort != 5080 {
		t.Fatalf("slot0 => %+v; want 10.1.10.2 / 10.1.10.1 / 5080", id)
	}
}

func TestDeriveClientTunnelIdentity_HigherSlotMirrorsServer(t *testing.T) {
	id := DeriveClientTunnelIdentity(2, "", "", 0)
	// must mirror the server peer derivation for slot 2
	p := MlvpnPeer{Slot: 2}
	DerivePeerDefaults(&p)
	if id.ClientTunIP != p.ClientTunIP || id.ServerTunIP != p.ServerTunIP || id.RemotePort != p.Port {
		t.Fatalf("client/server disagree on slot 2: client=%+v peer=%+v", id, p)
	}
	if id.ClientTunIP != "10.1.12.2" || id.ServerTunIP != "10.1.12.1" || id.RemotePort != 5082 {
		t.Fatalf("slot2 => %+v", id)
	}
}

func TestDeriveClientTunnelIdentity_Overrides(t *testing.T) {
	id := DeriveClientTunnelIdentity(3, "172.16.0.9", "172.16.0.1", 7001)
	if id.ClientTunIP != "172.16.0.9" || id.ServerTunIP != "172.16.0.1" || id.RemotePort != 7001 {
		t.Fatalf("overrides not honored: %+v", id)
	}
}

func TestLoadClientTunnelIdentity_FromSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frameflow.settings")

	// default slot 0 when unset
	os.WriteFile(path, []byte("MLVPN_SERVER_IP=\"1.2.3.4\"\n"), 0600)
	id := LoadClientTunnelIdentity(path)
	if id.ServerTunIP != "10.1.10.1" || id.RemotePort != 5080 {
		t.Fatalf("unset slot => %+v", id)
	}

	// explicit slot derives
	os.WriteFile(path, []byte("MLVPN_SLOT=\"4\"\n"), 0600)
	id = LoadClientTunnelIdentity(path)
	if id.ClientTunIP != "10.1.14.2" || id.ServerTunIP != "10.1.14.1" || id.RemotePort != 5084 {
		t.Fatalf("slot 4 => %+v", id)
	}

	// explicit overrides win
	os.WriteFile(path, []byte("MLVPN_SLOT=\"4\"\nMLVPN_CLIENT_TUN_IP=\"10.9.9.9\"\nMLVPN_REMOTE_PORT=\"6000\"\n"), 0600)
	id = LoadClientTunnelIdentity(path)
	if id.ClientTunIP != "10.9.9.9" || id.RemotePort != 6000 || id.ServerTunIP != "10.1.14.1" {
		t.Fatalf("override mix => %+v", id)
	}
}
