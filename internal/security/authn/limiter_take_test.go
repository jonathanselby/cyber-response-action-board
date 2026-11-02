package authn

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Take never admits more than the bound, even under concurrency.
func TestTakeIsAStrictBoundUnderConcurrency(t *testing.T) {
	l := &FailureLimiter{MaxFailures: 60, Window: time.Minute}
	now := time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC)
	var admitted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Take("siem", now) {
				admitted.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := admitted.Load(); got != 60 {
		t.Fatalf("admitted %d, want exactly 60", got)
	}
	if !l.Take("siem", now.Add(time.Minute)) {
		t.Fatal("not admitted after the window")
	}
}
