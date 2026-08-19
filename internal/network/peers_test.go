package network

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDerivePeerDefaults_Slot0MatchesLegacy(t *testing.T) {
	p := MlvpnPeer{Slot: 0}
	DerivePeerDefaults(&p)
	if p.ServerTunIP != "10.1.10.1" || p.ClientTunIP != "10.1.10.2" || p.Port != 5080 {
		t.Fatalf("slot 0 derived %s/%s port %d; want 10.1.10.1/10.1.10.2 port 5080", p.ServerTunIP, p.ClientTunIP, p.Port)
	}
	if PeerInterface(0) != "mlvpn0" {
		t.Fatalf("PeerInterface(0)=%s", PeerInterface(0))
	}
}

func TestDerivePeerDefaults_HigherSlots(t *testing.T) {
	p := MlvpnPeer{Slot: 3}
	DerivePeerDefaults(&p)
	if p.ServerTunIP != "10.1.13.1" || p.ClientTunIP != "10.1.13.2" || p.Port != 5083 {
		t.Fatalf("slot 3 derived %s/%s port %d; want 10.1.13.1/10.1.13.2 port 5083", p.ServerTunIP, p.ClientTunIP, p.Port)
	}
	if PeerInterface(3) != "mlvpn3" || PeerServiceInstance(3) != "frameflow-mlvpn@3.service" {
		t.Fatalf("iface/svc: %s / %s", PeerInterface(3), PeerServiceInstance(3))
	}
}

func TestDerivePeerDefaults_ExplicitOverrideKept(t *testing.T) {
	p := MlvpnPeer{Slot: 5, ClientTunIP: "192.168.9.9", ServerTunIP: "192.168.9.1", Port: 6000}
	DerivePeerDefaults(&p)
	if p.ClientTunIP != "192.168.9.9" || p.ServerTunIP != "192.168.9.1" || p.Port != 6000 {
		t.Fatalf("overrides not preserved: %+v", p)
	}
}

func TestValidatePeers_DuplicatesAndBounds(t *testing.T) {
	mk := func(peers ...MlvpnPeer) []MlvpnPeer {
		for i := range peers {
			DerivePeerDefaults(&peers[i])
		}
		return peers
	}
	cases := []struct {
		name    string
		peers   []MlvpnPeer
		wantErr bool
	}{
		{"empty ok", mk(), false},
		{"two distinct ok", mk(MlvpnPeer{Slot: 0, Name: "a"}, MlvpnPeer{Slot: 1, Name: "b"}), false},
		{"missing name", mk(MlvpnPeer{Slot: 0}), true},
		{"bad name charset", mk(MlvpnPeer{Slot: 0, Name: "Bad_Name"}), true},
		{"bad name leading hyphen", mk(MlvpnPeer{Slot: 0, Name: "-lead"}), true},
		{"dup name", mk(MlvpnPeer{Slot: 0, Name: "same"}, MlvpnPeer{Slot: 1, Name: "same"}), true},
		{"dup slot", mk(MlvpnPeer{Slot: 0, Name: "a"}, MlvpnPeer{Slot: 0, Name: "b"}), true},
		{"slot too high", mk(MlvpnPeer{Slot: 100, Name: "a"}), true},
		{"negative slot", mk(MlvpnPeer{Slot: -1, Name: "a"}), true},
		{"reserved port override", mk(MlvpnPeer{Slot: 7, Name: "a", Port: 8890}), true},
		{"dup port override", mk(MlvpnPeer{Slot: 0, Name: "a"}, MlvpnPeer{Slot: 1, Name: "b", Port: 5080}), true},
		{"identical srv/cli", mk(MlvpnPeer{Slot: 2, Name: "a", ServerTunIP: "10.0.0.1", ClientTunIP: "10.0.0.1"}), true},
	}
	for _, c := range cases {
		err := validatePeers(c.peers)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: got err=%v wantErr=%v", c.name, err, c.wantErr)
		}
	}
}

func TestFindPeerByName(t *testing.T) {
	peers := []MlvpnPeer{{Slot: 0, Name: "lobby"}, {Slot: 1, Name: "backpack-02"}}
	if p, ok := FindPeerByName(peers, "LOBBY"); !ok || p.Slot != 0 {
		t.Fatalf("case-insensitive lookup failed: %+v ok=%v", p, ok)
	}
	if _, ok := FindPeerByName(peers, "nope"); ok {
		t.Fatal("unexpected match for absent name")
	}
}

func TestLoadPeers_MinimalAndSorted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.yaml")
	os.WriteFile(path, []byte(`# registry
peers:
  - slot: 2
    name: "c2"
    key: "kkk"
  - slot: 0
    name: "c0"
    key: "aaa"
`), 0600)

	peers, err := LoadPeers(path)
	if err != nil {
		t.Fatalf("LoadPeers: %v", err)
	}
	if len(peers) != 2 {
		t.Fatalf("want 2 peers, got %d", len(peers))
	}
	if peers[0].Slot != 0 || peers[1].Slot != 2 {
		t.Fatalf("not sorted by slot: %d,%d", peers[0].Slot, peers[1].Slot)
	}
	if peers[1].ServerTunIP != "10.1.12.1" || peers[1].Port != 5082 {
		t.Fatalf("slot 2 derivation wrong: %s port %d", peers[1].ServerTunIP, peers[1].Port)
	}
}

func TestLoadPeers_RejectsDuplicateSlot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.yaml")
	os.WriteFile(path, []byte("peers:\n  - slot: 1\n    name: alpha\n  - slot: 1\n    name: beta\n"), 0600)
	if _, err := LoadPeers(path); err == nil {
		t.Fatal("expected duplicate-slot error")
	}
}

func TestEnsurePeerKey_GeneratesPersistsPreservesComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.yaml")
	original := `# top comment must survive
peers:
  - slot: 0
    name: "c0"   # inline comment
    key: ""
  - slot: 1
    name: "c1"
    key: "preset-key"
`
	os.WriteFile(path, []byte(original), 0600)

	// slot 0 empty -> generate + persist
	k0, err := EnsurePeerKey(path, 0, "")
	if err != nil {
		t.Fatalf("EnsurePeerKey slot0: %v", err)
	}
	if len(k0) != 32 {
		t.Fatalf("generated key length = %d, want 32 hex chars", len(k0))
	}

	after, _ := os.ReadFile(path)
	s := string(after)
	if !strings.Contains(s, "# top comment must survive") {
		t.Error("top comment lost")
	}
	if !strings.Contains(s, "inline comment") {
		t.Error("inline comment lost")
	}
	if !strings.Contains(s, k0) {
		t.Error("generated key not persisted")
	}
	if !strings.Contains(s, "preset-key") {
		t.Error("preset key on slot 1 disturbed")
	}

	// idempotent: reloading and re-ensuring returns the same persisted key
	peers, err := LoadPeers(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if peers[0].Key != k0 {
		t.Fatalf("persisted key mismatch on reload: %q vs %q", peers[0].Key, k0)
	}
	k0again, err := EnsurePeerKey(path, 0, peers[0].Key)
	if err != nil {
		t.Fatalf("re-ensure: %v", err)
	}
	if k0again != k0 {
		t.Fatalf("re-ensure changed key: %q -> %q", k0, k0again)
	}
}

func TestEnsurePeerKey_PresetKeptWhenNonEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.yaml")
	os.WriteFile(path, []byte("peers:\n  - slot: 4\n    key: \"already\"\n"), 0600)
	k, err := EnsurePeerKey(path, 4, "already")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if k != "already" {
		t.Fatalf("preset key changed: %q", k)
	}
}
