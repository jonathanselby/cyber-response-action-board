package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// Notifications (R-05, contract §3.5). CRAB contains the truth; notifications
// tell someone to look at CRAB. A notice says what kind of thing needs the
// person and, where they may read the incident, its ID. It carries no titles,
// summaries, compartment names or links, so it can never reveal restricted
// content and can never be used to approve anything.

const (
	TypeNotificationRequest = "incident.notification_request"
	TypeNotice              = "incident.notice"

	// SystemMaker is the maker of notices CRAB sends on its own.
	SystemMaker = "crab"

	NoticeAssignment = "assignment"
	NoticeDecision   = "decision_needed"
	NoticeDeadline   = "deadline"
	NoticeEscalation = "escalation"
	NoticeAccess     = "access_expiring"

	// NoticeNoRoute is the outcome when the person has no route: nothing is
	// sent and the item stays on their Command Summary.
	NoticeNoRoute = "no_route"
)

// NotificationRequest is recorded before any delivery attempt of an issued
// communication, so a request whose attempts never happened (the process
// stopped) is visible after restart.
type NotificationRequest struct {
	CommunicationID string   `json:"communication_id"`
	Channels        []string `json:"channels"`
	// ResendOf lists the earlier attempt records this request resends.
	ResendOf []string `json:"resend_of,omitempty"`
}

// Notice is one notice CRAB sent (or could not send) for a V1 event. It never
// holds an address.
type Notice struct {
	NoticeID         string    `json:"notice_id"`
	Kind             string    `json:"kind"`
	RecipientHumanID string    `json:"recipient_human_id,omitempty"`
	SharedChannel    string    `json:"shared_channel,omitempty"`
	Channel          string    `json:"channel,omitempty"`
	Status           string    `json:"status"`
	Detail           string    `json:"detail,omitempty"`
	SourceID         string    `json:"source_id"`
	SourceAt         time.Time `json:"source_at"`
	AttemptedAt      time.Time `json:"attempted_at"`
}

type NotificationRequestView struct {
	RecordID    string
	IncidentID  string
	Compartment string
	Request     NotificationRequest
	Maker       string
	RequestedAt time.Time
}

type NoticeView struct {
	IncidentID  string
	Compartment string
	Notice      Notice
}

func ValidateNotificationRequest(value NotificationRequest) error {
	if strings.TrimSpace(value.CommunicationID) == "" {
		return errors.New("notification request requires a communication")
	}
	if err := validateStringSet("channels", value.Channels, 1, MaxCommunicationChannels); err != nil {
		return err
	}
	return validateStringSet("resend of", value.ResendOf, 0, MaxCommunicationChannels*8)
}

func RecordNotificationRequest(scope Scope, recordID, maker string, now time.Time, value NotificationRequest) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateNotificationRequest(value); err != nil {
		return ledger.Record{}, err
	}
	return immediateRecord(scope, TypeNotificationRequest, recordID, maker, now, value)
}

func validNoticeKind(kind string) bool {
	switch kind {
	case NoticeAssignment, NoticeDecision, NoticeDeadline, NoticeEscalation, NoticeAccess:
		return true
	}
	return false
}

func ValidateNotice(value Notice) error {
	if strings.TrimSpace(value.NoticeID) == "" || strings.TrimSpace(value.SourceID) == "" || value.SourceAt.IsZero() || value.AttemptedAt.IsZero() {
		return errors.New("notice requires id, source and times")
	}
	if !validNoticeKind(value.Kind) {
		return errors.New("unsupported notice kind")
	}
	if (value.RecipientHumanID == "") == (value.SharedChannel == "") {
		return errors.New("notice needs exactly one of a recipient or a shared channel")
	}
	if value.SharedChannel != "" && value.Kind != NoticeEscalation {
		return errors.New("only escalation notices may go to a shared destination")
	}
	switch value.Status {
	case NotificationDelivered, NotificationFailed, NotificationSuppressed, NoticeNoRoute:
	default:
		return errors.New("notice status must be delivered, failed, suppressed or no_route")
	}
	if strings.Contains(value.Detail, "@") {
		return errors.New("notice detail must not carry an address")
	}
	return nil
}

func RecordNotice(scope Scope, now time.Time, value Notice) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateNotice(value); err != nil {
		return ledger.Record{}, err
	}
	value.SourceAt, value.AttemptedAt = value.SourceAt.UTC(), value.AttemptedAt.UTC()
	return immediateRecord(scope, TypeNotice, value.NoticeID, SystemMaker, now, value)
}

// NoticeID is deterministic per event kind, recipient and event key, so the
// same event never notifies the same person twice.
func NoticeID(kind, recipient, key string) string {
	digest := sha256.Sum256([]byte(kind + "|" + recipient + "|" + key))
	return "notice-" + hex.EncodeToString(digest[:12])
}

// NoticeText composes a person's notice. incidentID is empty when the person
// may not read the incident. count is used for decision notices only and
// counts only items the person may decide and read.
func NoticeText(kind, incidentID string, count int) string {
	on := ""
	if incidentID != "" {
		on = " on " + incidentID
	}
	switch kind {
	case NoticeAssignment:
		return "CRAB: you have a new assignment" + on + ". Sign in to CRAB."
	case NoticeDecision:
		if count == 1 {
			return "CRAB: 1 item" + on + " is waiting for your decision. Sign in to CRAB."
		}
		return fmt.Sprintf("CRAB: %d items%s are waiting for your decision. Sign in to CRAB.", count, on)
	case NoticeDeadline:
		return "CRAB: a deadline you own" + on + " needs attention. Sign in to CRAB."
	case NoticeEscalation:
		return "CRAB: an incident you are on" + on + " has a confirmed severity or phase change. Sign in to CRAB."
	case NoticeAccess:
		return "CRAB: temporary access you can renew" + on + " expires soon. Sign in to CRAB."
	}
	return "CRAB: something needs your attention. Sign in to CRAB."
}

// SharedEscalationText is the only notice a shared destination ever receives:
// content-free, with no incident ID.
func SharedEscalationText() string {
	return "CRAB: an incident has a confirmed severity or phase change. Sign in to CRAB."
}

// MaskAddress is the hint recorded in the ledger in place of an address:
// "j…@forensic.example" for email, "…789" otherwise.
func MaskAddress(address string) string {
	address = strings.TrimSpace(address)
	if at := strings.LastIndex(address, "@"); at > 0 {
		first, _ := utf8.DecodeRuneInString(address)
		return string(first) + "…" + address[at:]
	}
	if n := utf8.RuneCountInString(address); n > 3 {
		r := []rune(address)
		return "…" + string(r[n-3:])
	}
	return "…"
}

// DueDeadline is an obligation warning or deadline that has fallen due on an
// active clock, read from the timed wait list.
type DueDeadline struct {
	IncidentID, Mode, Compartment, ClockID, Kind, Owner string
	WakeAt                                              time.Time
}

// ExpiringAccess is a temporary membership expiring soon, with the people who
// can renew it (Sponsors for incident membership, Controllers for a
// restricted compartment).
type ExpiringAccess struct {
	HumanID, IncidentID, Compartment, Mode string
	ExpiresAt                              time.Time
	Renewers                               []string
}
