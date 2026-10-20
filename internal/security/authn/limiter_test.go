package authn

import (
	"testing"
	"time"
)

func TestFailureLimiterBoundsAttemptsAndRecoversAfterWindow(t *testing.T) {
	now := time.Date(2026, 10, 20, 2, 0, 0, 0, time.UTC)
	limiter := &FailureLimiter{MaxFailures: 3, Window: time.Minute, MaxKeys: 2}
	for i := 0; i < 3; i++ {
		if !limiter.Allow("human-1", now) {
			t.Fatalf("attempt %d unexpectedly blocked", i+1)
		}
		limiter.Failure("human-1", now)
	}
	if limiter.Allow("human-1", now) {
		t.Fatal("fourth attempt unexpectedly allowed")
	}
	if !limiter.Allow("human-1", now.Add(time.Minute)) {
		t.Fatal("attempt not restored after limiter window")
	}
	limiter.Failure("human-1", now.Add(time.Minute))
	limiter.Success("human-1")
	if !limiter.Allow("human-1", now.Add(time.Minute)) {
		t.Fatal("successful authentication did not clear failure state")
	}
}
