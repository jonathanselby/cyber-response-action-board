package domain

import "testing"

func TestNormaliseObservable(t *testing.T) {
	for _, tc := range []struct{ kind, in, want string }{
		{ObservableIP, "2001:0DB8:0:0:0:0:0:1", "2001:db8::1"},
		{ObservableIP, "::ffff:203.0.113.7", "203.0.113.7"},
		{ObservableIP, " 203.0.113.7 ", "203.0.113.7"},
		{ObservableDomain, "Evil.Example.", "evil.example"},
		{ObservableHostname, "FIN-WS-22", "fin-ws-22"},
		{ObservableURL, "HTTPS://Evil.Example/Login?a=B", "https://evil.example/Login?a=B"},
		{ObservableEmail, "Ceo@Evil.Example", "Ceo@evil.example"},
		{ObservableHash, "D41D8CD98F00B204E9800998ECF8427E", "d41d8cd98f00b204e9800998ecf8427e"},
	} {
		got, err := NormaliseObservable(tc.kind, tc.in)
		if err != nil || got != tc.want {
			t.Errorf("%s %q: got %q, %v; want %q", tc.kind, tc.in, got, err, tc.want)
		}
	}
	for _, tc := range []struct{ kind, in string }{
		{ObservableIP, "999.1.1.1"}, {ObservableIP, "fe80::1%eth0"}, {ObservableDomain, "localhost"}, {ObservableDomain, "-bad.example"},
		{ObservableHostname, "has space"}, {ObservableURL, "evil.example/path"}, {ObservableURL, "javascript:alert(1)"},
		{ObservableEmail, "no-at.example"}, {ObservableHash, "abc123"}, {ObservableHash, "zz1d8cd98f00b204e9800998ecf8427e"},
		{"asn", "AS64500"}, {ObservableIP, ""}, {ObservableDomain, "evil\x00.example"},
	} {
		if got, err := NormaliseObservable(tc.kind, tc.in); err == nil {
			t.Errorf("%s %q accepted as %q", tc.kind, tc.in, got)
		}
	}
}
