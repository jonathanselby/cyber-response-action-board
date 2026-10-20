package authn

import (
	"strings"
	"sync"
	"time"
)

const DefaultLimiterMaxKeys = 4096

type limitEntry struct {
	WindowStart time.Time
	Failures    int
}

// FailureLimiter is a deliberately small bounded authentication failure limiter.
// It stores no credentials and is not an audit history.
type FailureLimiter struct {
	mu          sync.Mutex
	MaxFailures int
	Window      time.Duration
	MaxKeys     int
	entries     map[string]limitEntry
}

func (l *FailureLimiter) Allow(key string, now time.Time) bool {
	key = strings.TrimSpace(key)
	if key == "" || l.MaxFailures <= 0 || l.Window <= 0 || now.IsZero() {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = make(map[string]limitEntry)
	}
	l.prune(now)
	entry, exists := l.entries[key]
	if !exists {
		maxKeys := l.MaxKeys
		if maxKeys <= 0 {
			maxKeys = DefaultLimiterMaxKeys
		}
		return len(l.entries) < maxKeys
	}
	if !now.Before(entry.WindowStart.Add(l.Window)) {
		delete(l.entries, key)
		return true
	}
	return entry.Failures < l.MaxFailures
}

func (l *FailureLimiter) Failure(key string, now time.Time) {
	key = strings.TrimSpace(key)
	if key == "" || now.IsZero() || l.MaxFailures <= 0 || l.Window <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = make(map[string]limitEntry)
	}
	l.prune(now)
	entry, exists := l.entries[key]
	if !exists || !now.Before(entry.WindowStart.Add(l.Window)) {
		maxKeys := l.MaxKeys
		if maxKeys <= 0 {
			maxKeys = DefaultLimiterMaxKeys
		}
		if len(l.entries) >= maxKeys {
			return
		}
		entry = limitEntry{WindowStart: now}
	}
	entry.Failures++
	l.entries[key] = entry
}

func (l *FailureLimiter) Success(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries != nil {
		delete(l.entries, strings.TrimSpace(key))
	}
}

func (l *FailureLimiter) prune(now time.Time) {
	for key, entry := range l.entries {
		if !now.Before(entry.WindowStart.Add(l.Window)) {
			delete(l.entries, key)
		}
	}
}
