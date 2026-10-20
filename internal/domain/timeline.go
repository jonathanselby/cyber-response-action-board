package domain

import (
	"errors"
	"strings"
	"time"
)

type AwarenessTime struct {
	Subject string    `json:"subject"`
	At      time.Time `json:"at"`
}

type TimelineEntry struct {
	Sequence    uint64
	RecordID    string
	RecordType  string
	Governance  string
	Compartment string
	Summary     string
	OccurredAt  *time.Time
	FlaggedAt   *time.Time
	RecordedAt  time.Time
	Awareness   []AwarenessTime
	TimeStatus  string
}

func ValidateFactTiming(fact Fact) error {
	if fact.OccurredAt != nil && fact.OccurredAt.IsZero() {
		return errors.New("fact occurrence time cannot be zero")
	}
	if fact.FlaggedAt != nil && fact.FlaggedAt.IsZero() {
		return errors.New("fact flagged time cannot be zero")
	}
	seen := map[string]struct{}{}
	for _, awareness := range fact.Awareness {
		if strings.TrimSpace(awareness.Subject) == "" || awareness.At.IsZero() {
			return errors.New("awareness subject and time are required")
		}
		if _, exists := seen[awareness.Subject]; exists {
			return errors.New("duplicate awareness subject")
		}
		seen[awareness.Subject] = struct{}{}
	}
	return nil
}
