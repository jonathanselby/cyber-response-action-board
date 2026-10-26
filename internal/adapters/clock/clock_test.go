package clock

import (
	"testing"
	"time"
)

func TestWeakerNeverStrengthens(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{Trusted, Trusted, Trusted},
		{Trusted, Drifting, Drifting},
		{Drifting, Trusted, Drifting},
		{Trusted, Untrusted, Untrusted},
		{Untrusted, Trusted, Untrusted},
		{Drifting, Untrusted, Untrusted},
		{"", Trusted, Untrusted},
		{"bogus", Trusted, Untrusted},
	}
	for _, c := range cases {
		if got := Weaker(c.a, c.b); got != c.want {
			t.Errorf("Weaker(%q,%q)=%q want %q", c.a, c.b, got, c.want)
		}
	}
}

// T-195 / T-196: classification of the host discipline.
func TestHostClassification(t *testing.T) {
	cases := []struct {
		synced bool
		err    time.Duration
		want   string
	}{
		{false, 0, Untrusted},
		{true, 10 * time.Millisecond, Trusted},
		{true, DriftThreshold, Trusted},
		{true, DriftThreshold + time.Millisecond, Drifting},
	}
	for _, c := range cases {
		h := &Host{probe: func() (bool, time.Duration, string) { return c.synced, c.err, "test" }}
		if got := h.Read().Status; got != c.want {
			t.Errorf("synced=%v err=%v status=%q want %q", c.synced, c.err, got, c.want)
		}
	}
}

func TestHostCachesProbe(t *testing.T) {
	calls := 0
	h := &Host{probe: func() (bool, time.Duration, string) { calls++; return true, 0, "test" }}
	h.Read()
	h.Read()
	if calls != 1 {
		t.Fatalf("probe called %d times", calls)
	}
}

func TestHostProbeRunsWithoutPrivilege(t *testing.T) {
	r := NewHost().Read()
	if r.Status != Trusted && r.Status != Drifting && r.Status != Untrusted {
		t.Fatalf("unexpected status %q", r.Status)
	}
	if r.Now.IsZero() || r.Detail == "" {
		t.Fatalf("incomplete reading %+v", r)
	}
}
