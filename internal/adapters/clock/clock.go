// Package clock is CRAB's single time authority (UAT 2 item 1, T-194 to T-196).
//
// It returns the current time together with how far CRAB can stand behind it.
// CRAB keeps operating when the host clock cannot be verified; it simply stops
// asserting that time is trusted.
package clock

import (
	"sync"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

const (
	Trusted   = ledger.TimeTrusted
	Drifting  = ledger.TimeDrifting
	Untrusted = ledger.TimeUntrusted
)

// Reading is one observation of the clock.
type Reading struct {
	Now    time.Time
	Status string
	Detail string
}

// Clock is implemented by the host clock and by fixed clocks in tests.
type Clock interface {
	Read() Reading
}

// Weaker returns the less trustworthy of two time statuses.
func Weaker(a, b string) string { return ledger.WeakerTimeStatus(a, b) }

func normal(s string) string { return ledger.WeakerTimeStatus(s, Trusted) }

// DriftThreshold is the estimated error above which a synchronised clock is
// reported as drifting.
const DriftThreshold = 250 * time.Millisecond

// cacheFor bounds how often the host is asked.
const cacheFor = 5 * time.Second

// Host reads the host kernel's clock discipline.
type Host struct {
	mu     sync.Mutex
	at     time.Time
	cached Reading
	probe  func() (synced bool, estimatedError time.Duration, detail string)
}

// NewHost returns the host clock.
func NewHost() *Host { return &Host{probe: hostProbe} }

func (h *Host) Read() Reading {
	now := time.Now().UTC()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.at.IsZero() || now.Sub(h.at) >= cacheFor || now.Before(h.at) {
		h.cached = classify(h.probe())
		h.at = now
	}
	return Reading{Now: now, Status: h.cached.Status, Detail: h.cached.Detail}
}

func classify(synced bool, estimatedError time.Duration, detail string) Reading {
	switch {
	case !synced:
		return Reading{Status: Untrusted, Detail: detail}
	case estimatedError > DriftThreshold:
		return Reading{Status: Drifting, Detail: detail}
	}
	return Reading{Status: Trusted, Detail: detail}
}

// Fixed is a clock for tests and for replaying a known reading.
type Fixed struct {
	At     time.Time
	Status string
}

func (f Fixed) Read() Reading {
	at := f.At
	if at.IsZero() {
		at = time.Now()
	}
	return Reading{Now: at.UTC(), Status: normal(f.Status), Detail: "fixed clock"}
}
