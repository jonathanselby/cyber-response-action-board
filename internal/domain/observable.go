package domain

import (
	"encoding/hex"
	"errors"
	"net/netip"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// TypeObservable is a typed cyber observable recorded in an incident
// (contract addendum A2). It is an immutable Record: a status change is a new
// version of the same observable. There is no enrichment, lookup, feed or
// export.
const TypeObservable = "incident.observable"

// Observable types.
const (
	ObservableIP       = "ip"
	ObservableDomain   = "domain"
	ObservableURL      = "url"
	ObservableHostname = "hostname"
	ObservableEmail    = "email"
	ObservableHash     = "hash"
)

// Observable statuses.
const (
	ObservableSuspected = "suspected"
	ObservableConfirmed = "confirmed"
	ObservableBenign    = "benign"
	ObservableBlocked   = "blocked"
)

// ObservableTypes and ObservableStatuses are in display order.
var (
	ObservableTypes    = []string{ObservableIP, ObservableDomain, ObservableURL, ObservableHostname, ObservableEmail, ObservableHash}
	ObservableStatuses = []string{ObservableSuspected, ObservableConfirmed, ObservableBenign, ObservableBlocked}
)

const (
	maxObservableValue = 2048
	maxObservableText  = 500
)

// Observable is the payload of an incident.observable record. ObservableID is
// stable across versions; Type and Value never change between versions.
type Observable struct {
	ObservableID   string    `json:"observable_id"`
	Type           string    `json:"type"`
	Value          string    `json:"value"`
	Status         string    `json:"status"`
	Source         string    `json:"source"`
	FirstSeen      time.Time `json:"first_seen"`
	TimeConfidence string    `json:"time_confidence"`
	Note           string    `json:"note,omitempty"`
}

// ObservableView is the current version of one observable.
type ObservableView struct {
	RecordID    string
	IncidentID  string
	Mode        string
	Compartment string
	Version     int
	Maker       string
	RecordedAt  time.Time
	Observable  Observable
}

// ObservableSighting is the same normalised value in some incident. Whether it
// may be shown is decided by the application tier.
type ObservableSighting struct {
	IncidentID  string
	Compartment string
}

// NormaliseObservable validates value for its type and returns the canonical
// form used for storage and for matching across incidents.
func NormaliseObservable(kind, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("an observable value is required")
	}
	if utf8.RuneCountInString(value) > maxObservableValue || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", errors.New("an observable value is at most 2048 printable characters")
	}
	switch kind {
	case ObservableIP:
		addr, err := netip.ParseAddr(value)
		if err != nil || addr.Zone() != "" {
			return "", errors.New("not a valid IPv4 or IPv6 address")
		}
		return addr.Unmap().String(), nil
	case ObservableDomain:
		name, ok := hostName(value)
		if !ok || !strings.Contains(name, ".") {
			return "", errors.New("not a valid domain name")
		}
		return name, nil
	case ObservableHostname:
		name, ok := hostName(value)
		if !ok {
			return "", errors.New("not a valid hostname")
		}
		return name, nil
	case ObservableURL:
		u, err := url.Parse(value)
		if err != nil || u.Scheme == "" || u.Host == "" || strings.ContainsAny(value, " \t") {
			return "", errors.New("not a valid absolute URL")
		}
		u.Scheme = strings.ToLower(u.Scheme)
		u.Host = strings.ToLower(u.Host)
		return u.String(), nil
	case ObservableEmail:
		local, domain, ok := strings.Cut(value, "@")
		name, valid := hostName(domain)
		if !ok || local == "" || strings.ContainsAny(local, " @<>") || !valid || !strings.Contains(name, ".") {
			return "", errors.New("not a valid email address")
		}
		return local + "@" + name, nil
	case ObservableHash:
		h := strings.ToLower(value)
		if _, err := hex.DecodeString(h); err != nil {
			return "", errors.New("a file hash is hexadecimal")
		}
		switch len(h) {
		case 32, 40, 64, 128:
			return h, nil
		}
		return "", errors.New("a file hash is MD5, SHA-1, SHA-256 or SHA-512 (32, 40, 64 or 128 hex characters)")
	}
	return "", errors.New("observable type must be ip, domain, url, hostname, email or hash")
}

func hostName(value string) (string, bool) {
	name := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	if name == "" || len(name) > 253 {
		return "", false
	}
	for _, label := range strings.Split(name, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
				return "", false
			}
		}
	}
	return name, true
}

// ValidateObservable checks a complete observable payload.
func ValidateObservable(value Observable) error {
	if strings.TrimSpace(value.ObservableID) == "" {
		return errors.New("observable id is required")
	}
	normal, err := NormaliseObservable(value.Type, value.Value)
	if err != nil {
		return err
	}
	if normal != value.Value {
		return errors.New("observable value must be stored in normalised form")
	}
	switch value.Status {
	case ObservableSuspected, ObservableConfirmed, ObservableBenign, ObservableBlocked:
	default:
		return errors.New("observable status must be suspected, confirmed, benign or blocked")
	}
	if strings.TrimSpace(value.Source) == "" {
		return errors.New("observable source is required")
	}
	if utf8.RuneCountInString(value.Source) > maxObservableText || utf8.RuneCountInString(value.Note) > maxObservableText {
		return errors.New("observable source and note are at most 500 characters")
	}
	if value.FirstSeen.IsZero() {
		return errors.New("first seen time is required")
	}
	switch value.TimeConfidence {
	case ledger.TimeTrusted, ledger.TimeDrifting, ledger.TimeUntrusted:
	default:
		return errors.New("first seen time confidence must be trusted, drifting or untrusted")
	}
	return nil
}

// RecordObservable builds an incident.observable record.
func RecordObservable(scope Scope, recordID, maker string, now time.Time, value Observable) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	value.FirstSeen = value.FirstSeen.UTC()
	if err := ValidateObservable(value); err != nil {
		return ledger.Record{}, err
	}
	return immediateRecord(scope, TypeObservable, recordID, maker, now, value)
}

// NextObservableVersion returns the payload of a status change: the same
// observable, type, value and first-seen time, with a new status, source and
// note.
func NextObservableVersion(current Observable, status, source, note string) Observable {
	next := current
	next.Status, next.Source, next.Note = strings.TrimSpace(status), strings.TrimSpace(source), strings.TrimSpace(note)
	return next
}

// Critical action priorities for the management summary (addendum A3).
// Priority is free text; P1 is the form's default top priority.
var CriticalActionPriorities = []string{"P1", "CRITICAL"}

// ManagementSummary is the fixed management panel on the Command Summary,
// computed over incidents the viewer may see, live only.
type ManagementSummary struct {
	ActiveIncidents       int
	BySeverity            []SeverityCount
	OverdueCriticalAction int
	OverdueObligations    int
	DecisionsInWindow     int
	MedianDecisionLoop    *time.Duration // nil when there were no confirmed decisions
}

// SeverityCount is the number of active incidents at one severity.
type SeverityCount struct {
	Severity string
	Count    int
}

// DecisionLoopWindow is the period the median decision-loop time covers.
const DecisionLoopWindow = 7 * 24 * time.Hour
