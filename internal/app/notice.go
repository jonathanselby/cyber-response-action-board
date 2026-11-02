package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

// Notifications V1 (R-05, U-20, contract §3.5). Exactly five events notify,
// each to the person who must act:
//
//  1. assignment: I am made owner of an action or holder of an operational role;
//  2. decision needed: proposals I may decide and read are waiting, one notice
//     per person per incident per window, with a count;
//  3. deadline: an obligation warning or deadline falls due, to the clock owner,
//     read from the timed wait list;
//  4. material escalation: a confirmed severity or phase change, to members
//     and to shared destinations, only where the organisation enables it;
//  5. access needing action: temporary access expiring soon, to the Sponsor or
//     Controller who can renew it.
//
// Nothing else notifies. A notice never carries content beyond the kind of
// item and, where the person may read it, the incident ID.

// NoticePolicy is the organisation's notice settings.
type NoticePolicy struct {
	EscalationEnabled        bool
	EscalationToMembers      bool
	EscalationSharedChannels []string
	DecisionWindow           time.Duration
	AccessExpiryLead         time.Duration
}

func DefaultNoticePolicy() NoticePolicy {
	return NoticePolicy{DecisionWindow: 15 * time.Minute, AccessExpiryLead: 24 * time.Hour}
}

// ValidateNoticePolicy requires shared escalation destinations to be
// configured fixed channels: per-person channels never carry shared notices.
func ValidateNoticePolicy(p NoticePolicy, channels []NotificationChannel) error {
	if p.DecisionWindow <= 0 || p.AccessExpiryLead <= 0 {
		return errors.New("notice windows must be positive")
	}
	for _, name := range p.EscalationSharedChannels {
		ch, ok := findChannel(channels, name)
		if !ok {
			return fmt.Errorf("escalation shared channel %q is not configured", name)
		}
		if ch.PerRecipient {
			return fmt.Errorf("escalation shared channel %q is a per-person channel", name)
		}
	}
	if len(p.EscalationSharedChannels) > 0 && !p.EscalationEnabled {
		return errors.New("escalation shared channels need escalation_enabled")
	}
	return nil
}

func findChannel(channels []NotificationChannel, name string) (NotificationChannel, bool) {
	for _, ch := range channels {
		if ch.Name == name {
			return ch, true
		}
	}
	return NotificationChannel{}, false
}

// NoticeBackend is the data the notice pass reads and writes. It applies no policy.
type NoticeBackend interface {
	AppendNotice(context.Context, ledger.Record) error
	Cursor(context.Context) (uint64, time.Time, bool, error)
	SetCursor(context.Context, uint64, time.Time) error
	TailSeq(context.Context) (uint64, error)
	EntriesSince(context.Context, uint64, int) ([]ledger.Entry, error)
	NoticeExists(context.Context, string) (bool, error)
	LastNotice(ctx context.Context, kind, humanID, incidentID string) (time.Time, time.Time, bool, error)
	IncidentsWithPending(context.Context) ([]string, error)
	IncidentMembers(context.Context, string) ([]string, error)
	IncidentMode(context.Context, string) (string, bool, error)
	DueDeadlines(ctx context.Context, since, now time.Time, limit int) ([]domain.DueDeadline, error)
	ExpiringAccess(ctx context.Context, now, until time.Time) ([]domain.ExpiringAccess, error)
	Route(context.Context, string) (string, string, bool, error)
}

// NoticeService runs the notice pass.
type NoticeService struct {
	Backend   NoticeBackend
	People    PrincipalSource
	Approvals ApprovalsService
	Notifier  NotificationSender
	Channels  []NotificationChannel
	Policy    NoticePolicy
}

// NoticeReport counts what one pass did.
type NoticeReport struct {
	Delivered, Failed, Suppressed, NoRoute int
}

const (
	noticeEntryBatch    = 500
	noticeDeadlineBatch = 200
)

// Pass evaluates every event once. It is idempotent: each notice has a
// deterministic ID, so running it twice, or after a restart, sends nothing
// twice. The first pass only records where notices start, so history before
// notifications were enabled never notifies.
func (s NoticeService) Pass(ctx context.Context, now time.Time) (NoticeReport, error) {
	var report NoticeReport
	if s.Backend == nil || s.People == nil {
		return report, errors.New("notice service is not configured")
	}
	now = now.UTC()
	cursor, started, ok, err := s.Backend.Cursor(ctx)
	if err != nil {
		return report, err
	}
	if !ok {
		tail, err := s.Backend.TailSeq(ctx)
		if err != nil {
			return report, err
		}
		return report, s.Backend.SetCursor(ctx, tail, now)
	}
	if err := s.ledgerEvents(ctx, &report, cursor, started, now); err != nil {
		return report, err
	}
	if err := s.decisionEvents(ctx, &report, now); err != nil {
		return report, err
	}
	if err := s.deadlineEvents(ctx, &report, started, now); err != nil {
		return report, err
	}
	return report, s.accessEvents(ctx, &report, now)
}

// ledgerEvents reads new ledger entries for events 1 and 4.
func (s NoticeService) ledgerEvents(ctx context.Context, report *NoticeReport, cursor uint64, started, now time.Time) error {
	for {
		entries, err := s.Backend.EntriesSince(ctx, cursor, noticeEntryBatch)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := s.ledgerEvent(ctx, report, entry.Record, now); err != nil {
				return err
			}
			cursor = entry.Seq
		}
		if err := s.Backend.SetCursor(ctx, cursor, started); err != nil {
			return err
		}
		if len(entries) < noticeEntryBatch {
			return nil
		}
	}
}

func (s NoticeService) ledgerEvent(ctx context.Context, report *NoticeReport, r ledger.Record, now time.Time) error {
	if r.Status != "confirmed" || r.IncidentID == "" {
		return nil
	}
	actors := map[string]bool{r.Maker: true, r.Confirmer: true}
	switch r.RecordType {
	case domain.TypeAction:
		data, err := domain.DecodeGovernedData(r.Data)
		if err != nil {
			return nil
		}
		var a domain.Action
		if decodeJSON(data.Payload, &a) != nil || a.OwnerHumanID == "" || actors[a.OwnerHumanID] {
			return nil
		}
		return s.notify(ctx, report, event{kind: domain.NoticeAssignment, incidentID: r.IncidentID, mode: r.IncidentMode, compartment: r.Compartment, recipient: a.OwnerHumanID, sourceID: r.RecordID, sourceAt: r.Timestamp, key: r.RecordID}, now)
	case domain.TypeOperationalRoleAssignment:
		data, err := domain.DecodeGovernedData(r.Data)
		if err != nil {
			return nil
		}
		var a domain.OperationalRoleAssignment
		if decodeJSON(data.Payload, &a) != nil || a.HumanID == "" || actors[a.HumanID] {
			return nil
		}
		return s.notify(ctx, report, event{kind: domain.NoticeAssignment, incidentID: r.IncidentID, mode: r.IncidentMode, compartment: r.Compartment, recipient: a.HumanID, sourceID: r.RecordID, sourceAt: r.Timestamp, key: r.RecordID}, now)
	case domain.TypeSeverityChange, domain.TypeStandDown, domain.TypeClosure, domain.TypeReopening:
		if !s.Policy.EscalationEnabled || r.Compartment != "GENERAL" {
			return nil
		}
		if s.Policy.EscalationToMembers {
			members, err := s.Backend.IncidentMembers(ctx, r.IncidentID)
			if err != nil {
				return err
			}
			for _, m := range members {
				if actors[m] {
					continue
				}
				if err := s.notify(ctx, report, event{kind: domain.NoticeEscalation, incidentID: r.IncidentID, mode: r.IncidentMode, compartment: "GENERAL", recipient: m, sourceID: r.RecordID, sourceAt: r.Timestamp, key: r.RecordID}, now); err != nil {
					return err
				}
			}
		}
		for _, shared := range s.Policy.EscalationSharedChannels {
			if err := s.notifyShared(ctx, report, r, shared, now); err != nil {
				return err
			}
		}
	}
	return nil
}

// decisionEvents notifies each member who may decide and read pending items,
// at most once per incident per window, and only when something arrived since
// their last notice.
func (s NoticeService) decisionEvents(ctx context.Context, report *NoticeReport, now time.Time) error {
	incidents, err := s.Backend.IncidentsWithPending(ctx)
	if err != nil {
		return err
	}
	for _, incidentID := range incidents {
		mode, found, err := s.Backend.IncidentMode(ctx, incidentID)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		members, err := s.Backend.IncidentMembers(ctx, incidentID)
		if err != nil {
			return err
		}
		for _, human := range members {
			principal, usable, err := s.principal(ctx, human, now)
			if err != nil {
				return err
			}
			if !usable {
				continue
			}
			count, latest := s.decidable(ctx, principal, incidentID, now)
			if count == 0 {
				continue
			}
			lastAt, lastSource, had, err := s.Backend.LastNotice(ctx, domain.NoticeDecision, human, incidentID)
			if err != nil {
				return err
			}
			if had && (!latest.After(lastSource) || now.Sub(lastAt) < s.Policy.DecisionWindow) {
				continue
			}
			e := event{kind: domain.NoticeDecision, incidentID: incidentID, mode: mode, compartment: "GENERAL", recipient: human, sourceID: incidentID, sourceAt: latest, key: incidentID + "|" + latest.UTC().Format(time.RFC3339Nano), count: count}
			if err := s.notify(ctx, report, e, now); err != nil {
				return err
			}
		}
	}
	return nil
}

// decidable counts the incident's pending items the person may decide and
// read, and returns when the newest was proposed. Items they may not read are
// never counted, so a count reveals nothing.
func (s NoticeService) decidable(ctx context.Context, principal authz.Principal, incidentID string, now time.Time) (int, time.Time) {
	queue, err := s.Approvals.Queue(ctx, principal, incidentID, now)
	if err != nil {
		return 0, time.Time{}
	}
	var count int
	var latest time.Time
	for _, item := range queue.Waiting {
		if item.IncidentID != incidentID || !item.CanDecide {
			continue
		}
		compartment := item.Compartment
		if compartment == "" {
			compartment = "GENERAL"
		}
		scope := authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}
		if !authz.Authorise(principal, authz.ActionRead, scope).Allowed {
			continue
		}
		// Access changes have their own deciders; everything else needs confirm.
		if item.Kind != "access" && !authz.Authorise(principal, authz.ActionConfirm, scope).Allowed {
			continue
		}
		count++
		if item.ProposedAt.After(latest) {
			latest = item.ProposedAt
		}
	}
	return count, latest
}

// deadlineEvents notifies clock owners from the timed wait list.
func (s NoticeService) deadlineEvents(ctx context.Context, report *NoticeReport, started, now time.Time) error {
	due, err := s.Backend.DueDeadlines(ctx, started, now, noticeDeadlineBatch)
	if err != nil {
		return err
	}
	for _, d := range due {
		owner := strings.TrimSpace(d.Owner)
		if owner == "" {
			continue
		}
		key := d.ClockID + "|" + d.Kind + "|" + d.WakeAt.Format(time.RFC3339Nano)
		e := event{kind: domain.NoticeDeadline, incidentID: d.IncidentID, mode: d.Mode, compartment: d.Compartment, recipient: owner, sourceID: d.ClockID, sourceAt: d.WakeAt, key: key}
		if err := s.notify(ctx, report, e, now); err != nil {
			return err
		}
	}
	return nil
}

// accessEvents notifies the people who can renew temporary access that is
// about to expire.
func (s NoticeService) accessEvents(ctx context.Context, report *NoticeReport, now time.Time) error {
	expiring, err := s.Backend.ExpiringAccess(ctx, now, now.Add(s.Policy.AccessExpiryLead))
	if err != nil {
		return err
	}
	for _, a := range expiring {
		key := a.HumanID + "|" + a.IncidentID + "|" + a.Compartment + "|" + a.ExpiresAt.Format(time.RFC3339Nano)
		for _, renewer := range a.Renewers {
			e := event{kind: domain.NoticeAccess, incidentID: a.IncidentID, mode: a.Mode, compartment: "GENERAL", recipient: renewer, sourceID: a.IncidentID + "/" + a.HumanID, sourceAt: a.ExpiresAt, key: key}
			if err := s.notify(ctx, report, e, now); err != nil {
				return err
			}
		}
	}
	return nil
}

type event struct {
	kind, incidentID, mode, compartment, recipient, sourceID, key string
	sourceAt                                                      time.Time
	count                                                         int
}

// principal loads a person if their account is usable now. Nothing is sent to
// an account that is not usable.
func (s NoticeService) principal(ctx context.Context, human string, now time.Time) (authz.Principal, bool, error) {
	state, legacy, err := s.People.IdentityState(ctx, human)
	if err != nil {
		return authz.Principal{}, false, err
	}
	if !access.Usable(state, legacy, now) {
		return authz.Principal{}, false, nil
	}
	p, err := s.People.PrincipalAt(ctx, human, true, now)
	if err != nil {
		return authz.Principal{}, false, err
	}
	return p, true, nil
}

// notify sends one notice to one person over their route and records it.
func (s NoticeService) notify(ctx context.Context, report *NoticeReport, e event, now time.Time) error {
	id := domain.NoticeID(e.kind, e.recipient, e.key)
	if exists, err := s.Backend.NoticeExists(ctx, id); err != nil || exists {
		return err
	}
	notice := domain.Notice{NoticeID: id, Kind: e.kind, RecipientHumanID: e.recipient, SourceID: e.sourceID, SourceAt: e.sourceAt, AttemptedAt: now}
	principal, usable, err := s.principal(ctx, e.recipient, now)
	if err != nil {
		return err
	}
	state, _, _ := s.People.IdentityState(ctx, e.recipient)
	switch {
	case !usable && state.Found:
		return nil
	case !usable:
		// Not a person CRAB knows (for example a clock owner written as a
		// team name): nobody to send to.
		notice.Status, notice.Detail = domain.NoticeNoRoute, "owner is not a CRAB account"
	default:
		shown := ""
		if authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: e.incidentID, Compartment: "GENERAL"}).Allowed {
			shown = e.incidentID
		}
		notice.Channel, notice.Status, notice.Detail = s.sendToRoute(ctx, e.recipient, e.mode, domain.NoticeText(e.kind, shown, e.count))
	}
	return s.record(ctx, report, e.incidentID, e.mode, e.compartment, notice, now)
}

func (s NoticeService) sendToRoute(ctx context.Context, human, mode, text string) (channel, status, detail string) {
	name, address, ok, err := s.Backend.Route(ctx, human)
	if err != nil {
		return "", domain.NotificationFailed, "route unavailable"
	}
	if !ok {
		return "", domain.NoticeNoRoute, "no notification route"
	}
	ch, configured := findChannel(s.Channels, name)
	if !configured || !ch.PerRecipient {
		return name, domain.NotificationFailed, "route channel not configured"
	}
	if s.Notifier == nil {
		return name, domain.NotificationFailed, "notification adapter unavailable"
	}
	attempt := s.Notifier.Deliver(ctx, ch, NotificationMessage{Mode: mode, Body: text, To: address, Ref: human})
	return name, attempt.Status, safeDetail(attempt.Detail, address)
}

// notifyShared sends the content-free escalation notice to a shared
// destination. It carries no incident ID.
func (s NoticeService) notifyShared(ctx context.Context, report *NoticeReport, r ledger.Record, name string, now time.Time) error {
	id := domain.NoticeID(domain.NoticeEscalation, "shared:"+name, r.RecordID)
	if exists, err := s.Backend.NoticeExists(ctx, id); err != nil || exists {
		return err
	}
	notice := domain.Notice{NoticeID: id, Kind: domain.NoticeEscalation, SharedChannel: name, Channel: name, SourceID: r.RecordID, SourceAt: r.Timestamp, AttemptedAt: now}
	ch, ok := findChannel(s.Channels, name)
	switch {
	case !ok || ch.PerRecipient:
		notice.Status, notice.Detail = domain.NotificationFailed, "channel not configured"
	case s.Notifier == nil:
		notice.Status, notice.Detail = domain.NotificationFailed, "notification adapter unavailable"
	default:
		attempt := s.Notifier.Deliver(ctx, ch, NotificationMessage{Mode: r.IncidentMode, Body: domain.SharedEscalationText()})
		notice.Status, notice.Detail = attempt.Status, safeDetail(attempt.Detail, "")
	}
	return s.record(ctx, report, r.IncidentID, r.IncidentMode, "GENERAL", notice, now)
}

func (s NoticeService) record(ctx context.Context, report *NoticeReport, incidentID, mode, compartment string, notice domain.Notice, now time.Time) error {
	if notice.Status != domain.NotificationDelivered && notice.Status != domain.NotificationFailed && notice.Status != domain.NotificationSuppressed && notice.Status != domain.NoticeNoRoute {
		notice.Status, notice.Detail = domain.NotificationFailed, "notification adapter returned invalid status"
	}
	if compartment == "" {
		compartment = "GENERAL"
	}
	rec, err := domain.RecordNotice(domain.Scope{IncidentID: incidentID, Mode: mode, Compartment: compartment}, now, notice)
	if err != nil {
		return err
	}
	if err := s.Backend.AppendNotice(ctx, rec); err != nil {
		return err
	}
	switch notice.Status {
	case domain.NotificationDelivered:
		report.Delivered++
	case domain.NotificationSuppressed:
		report.Suppressed++
	case domain.NoticeNoRoute:
		report.NoRoute++
	default:
		report.Failed++
	}
	return nil
}

func decodeJSON(raw []byte, v any) error { return json.Unmarshal(raw, v) }

// safeDetail keeps addresses out of recorded details.
func safeDetail(detail, address string) string {
	if address != "" && strings.Contains(strings.ToLower(detail), strings.ToLower(address)) || strings.Contains(detail, "@") {
		return "delivery failed"
	}
	return detail
}

// ---- Routes ----

var (
	ErrRouteChannel        = errors.New("choose a notification channel that reaches a person")
	ErrRouteAddressRefused = errors.New("that address is not accepted by the channel")
	ErrRoutePerson         = errors.New("that person cannot have a notification route")
)

// RouteBackend stores routes. The address is supporting state; SetRoute
// appends the masked audit record in the same transaction.
type RouteBackend interface {
	Route(context.Context, string) (string, string, bool, error)
	SetRoute(ctx context.Context, audit ledger.Record, humanID, channel, address string, now time.Time) error
	RoutedPeople(context.Context) (map[string]string, error)
	DecisionMakersAndCustodians(context.Context) ([]string, error)
}

// RouteService sets and shows notification routes.
type RouteService struct {
	Backend  RouteBackend
	People   PrincipalSource
	Notifier NotificationSender
	Channels []NotificationChannel
}

// RouteViewer is the person looking at a route.
type RouteViewer = authz.Principal

// RouteView is what a person sees of a route: never the address, only a hint.
type RouteView struct {
	HumanID, Channel, Hint string
	Channels               []string
	CanEdit                bool
}

// PersonChannels lists the channels that reach a person.
func (s RouteService) PersonChannels() []string {
	var out []string
	for _, ch := range s.Channels {
		if ch.PerRecipient {
			out = append(out, ch.Name)
		}
	}
	return out
}

func mayManageRoute(actor authz.Principal, humanID string) bool {
	return actor.Authenticated && (actor.HumanID == humanID || actor.Custodian)
}

// View returns a person's route as they or a Custodian may see it.
func (s RouteService) View(ctx context.Context, actor authz.Principal, humanID string) (RouteView, error) {
	if !mayManageRoute(actor, humanID) {
		return RouteView{}, ErrAuthorisationDenied
	}
	v := RouteView{HumanID: humanID, Channels: s.PersonChannels(), CanEdit: true}
	channel, address, ok, err := s.Backend.Route(ctx, humanID)
	if err != nil {
		return RouteView{}, err
	}
	if ok {
		v.Channel, v.Hint = channel, domain.MaskAddress(address)
	}
	return v, nil
}

// Set sets or (with an empty address) clears a person's route. The person
// may set their own; a Custodian may set anyone's. It is recorded, not
// dual-controlled: a route carries no content and approves nothing.
func (s RouteService) Set(ctx context.Context, actor authz.Principal, humanID, channel, address string, now time.Time) error {
	humanID, channel, address = strings.TrimSpace(humanID), strings.TrimSpace(channel), strings.TrimSpace(address)
	if !mayManageRoute(actor, humanID) {
		return ErrAuthorisationDenied
	}
	if s.Backend == nil || s.People == nil {
		return errors.New("notification routes are not configured")
	}
	state, legacy, err := s.People.IdentityState(ctx, humanID)
	if err != nil {
		return err
	}
	if !access.Usable(state, legacy, now) && !(state.Found && state.Status == access.IdentitySuspended) {
		return ErrRoutePerson
	}
	change := access.NotificationRouteChange{HumanID: humanID, Cleared: address == ""}
	if address != "" {
		ch, ok := findChannel(s.Channels, channel)
		if !ok || !ch.PerRecipient {
			return ErrRouteChannel
		}
		if s.Notifier == nil || !s.Notifier.AcceptsAddress(ch, address) {
			return ErrRouteAddressRefused
		}
		change.Channel, change.Hint = channel, domain.MaskAddress(address)
	} else {
		channel = ""
	}
	digest := sha256.Sum256([]byte(humanID + "|" + now.UTC().Format(time.RFC3339Nano)))
	rec, err := access.NotificationRouteRecord("route-"+hex.EncodeToString(digest[:12]), actor.HumanID, change, now)
	if err != nil {
		return err
	}
	return s.Backend.SetRoute(ctx, rec, humanID, channel, address, now)
}

// Readiness warns when Majors or Custodians have no route.
func (s RouteService) Readiness(ctx context.Context) Check {
	if len(s.PersonChannels()) == 0 {
		return Check{Name: "notification_routes", Status: "pass", Message: "no per-person notification channel is configured; notices are recorded as no route"}
	}
	if s.Backend == nil {
		return Check{Name: "notification_routes", Status: "fail", Message: "notification routes are not configured"}
	}
	people, err := s.Backend.DecisionMakersAndCustodians(ctx)
	if err != nil {
		return Check{Name: "notification_routes", Status: "fail", Message: err.Error()}
	}
	routed, err := s.Backend.RoutedPeople(ctx)
	if err != nil {
		return Check{Name: "notification_routes", Status: "fail", Message: err.Error()}
	}
	missing := 0
	for _, h := range people {
		if _, ok := routed[h]; !ok {
			missing++
		}
	}
	if missing > 0 {
		return Check{Name: "notification_routes", Status: "warn", Message: fmt.Sprintf("%d Majors/Custodians have no notification route", missing)}
	}
	return Check{Name: "notification_routes", Status: "pass", Message: "every Major and Custodian has a notification route"}
}

// NoticeLine is one notice as a person sees it on the Communications screen.
type NoticeLine struct {
	Kind, Recipient, Channel, Status, StatusText string
	At                                           time.Time
}

// NoticeLines words the incident's notices, keeping only those in
// compartments the person may read.
func NoticeLines(principal authz.Principal, notices []domain.NoticeView) []NoticeLine {
	var out []NoticeLine
	for _, n := range notices {
		if !authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: n.IncidentID, Compartment: n.Compartment}).Allowed {
			continue
		}
		to := n.Notice.RecipientHumanID
		if n.Notice.SharedChannel != "" {
			to = "shared destination"
		}
		status := n.Notice.Status
		text := DeliveryStatusText(status)
		if status == domain.NoticeNoRoute {
			status, text = "no route", "Nothing sent: no notification route. The item stays on their Command Summary."
		}
		if n.Notice.Detail != "" && n.Notice.Status == domain.NotificationFailed {
			text += " (" + n.Notice.Detail + ")"
		}
		out = append(out, NoticeLine{Kind: NoticeKindText(n.Notice.Kind), Recipient: to, Channel: n.Notice.Channel, Status: status, StatusText: text, At: n.Notice.AttemptedAt})
	}
	return out
}

// NoticeKindText names a notice kind for people.
func NoticeKindText(kind string) string {
	switch kind {
	case domain.NoticeAssignment:
		return "New assignment"
	case domain.NoticeDecision:
		return "Decision needed"
	case domain.NoticeDeadline:
		return "Deadline"
	case domain.NoticeEscalation:
		return "Severity or phase change"
	case domain.NoticeAccess:
		return "Access expiring"
	}
	return kind
}
