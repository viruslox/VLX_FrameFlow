package cameraman

import "testing"

func TestBondingServerTunIP(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]string
		want string
	}{
		{"unset defaults to legacy", map[string]string{}, "10.1.10.1"},
		{"slot 0", map[string]string{"MLVPN_SLOT": "0"}, "10.1.10.1"},
		{"slot 5 derives", map[string]string{"MLVPN_SLOT": "5"}, "10.1.15.1"},
		{"non-numeric slot falls back", map[string]string{"MLVPN_SLOT": "garbage"}, "10.1.10.1"},
		{"explicit override wins", map[string]string{"MLVPN_SERVER_TUN_IP": "192.0.2.1", "MLVPN_SLOT": "5"}, "192.0.2.1"},
	}
	for _, c := range cases {
		if got := bondingServerTunIP(c.in); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}
