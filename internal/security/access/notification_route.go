package access

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// RecordNotificationRoute audits a change to a person's notification route.
// It names who changed whose route, the channel and a masked hint. The
// address itself is security-supporting state and never enters the ledger.
// A route carries no content and approves nothing, so it is recorded, not
// dual-controlled, and it is never a credential-recovery or verification
// route (AC-12).
const RecordNotificationRoute = "security.notification_route_changed"

// NotificationRouteChange is the payload of a route audit record.
type NotificationRouteChange struct {
	HumanID string `json:"human_id"`
	Channel string `json:"channel,omitempty"`
	Hint    string `json:"hint,omitempty"`
	Cleared bool   `json:"cleared,omitempty"`
}

// NotificationRouteRecord builds the audit record. maker is the person who
// made the change (the person themselves or a Custodian).
func NotificationRouteRecord(recordID, maker string, change NotificationRouteChange, now time.Time) (ledger.Record, error) {
	if strings.TrimSpace(recordID) == "" || strings.TrimSpace(maker) == "" || strings.TrimSpace(change.HumanID) == "" || now.IsZero() {
		return ledger.Record{}, errors.New("route audit requires record id, maker, person and time")
	}
	if change.Cleared != (change.Channel == "") || strings.Contains(change.Hint, "@") && !strings.Contains(change.Hint, "…") {
		return ledger.Record{}, errors.New("route audit must carry a channel and masked hint, or clear the route")
	}
	raw, err := json.Marshal(change)
	if err != nil {
		return ledger.Record{}, err
	}
	return ledger.Record{
		SchemaVersion:  1,
		Timestamp:      now.UTC(),
		TimeStatus:     ledger.TimeTrusted,
		IncidentMode:   "system",
		WriteSemantic:  "record",
		RecordType:     RecordNotificationRoute,
		RecordID:       recordID,
		Status:         "recorded",
		Maker:          maker,
		Compartment:    "SYSTEM",
		Data:           raw,
		MakerSignature: ledger.Signature{MissingReason: "maker authenticated session"},
	}, nil
}
