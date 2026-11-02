package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

// Approvals queue (F-7, R-22). Which pending changes a person sees, which
// they may decide, and how each is worded are application policy; the
// composition tier only lists what each store holds.

// DisplayTime is how times are written for people.
const DisplayTime = "2 Jan 2006 15:04 UTC"

// Field is one labelled value of a proposal.
type Field struct{ Name, Value string }

// PendingProposal is one pending governed change as a store holds it.
type PendingProposal struct {
	Kind, ScopeID, RecordType, RecordID, Compartment, Version, Maker string
	RelatedIncidentID                                                string // relationships only
	ProposedAt                                                       time.Time
}

// PersonLabel is how a person is named on an approval.
type PersonLabel struct {
	DisplayName, Organisation, Role string
	External                        bool
}

// IncidentRef is an incident the person may see.
type IncidentRef struct{ IncidentID, Title, Mode, Severity string }

// ApprovalsBackend lists pending proposals from each store. It applies no
// policy: the service decides what this person may see and decide.
type ApprovalsBackend interface {
	Incident(ctx context.Context, principal authz.Principal, incidentID string) (IncidentRef, bool)
	PendingInIncident(ctx context.Context, principal authz.Principal, incidentID, compartment string) []PendingProposal
	PendingRelationships(ctx context.Context, incidentID string) []PendingProposal
	PendingAccessForIncident(ctx context.Context, incidentID string) []PendingProposal
	PendingDeclarations(ctx context.Context) ([]PendingProposal, map[string]string)
	PendingSystemObligations(ctx context.Context) []PendingProposal
	PendingOrganisation(ctx context.Context, principal authz.Principal) []PendingProposal
	PendingAccessAdministration(ctx context.Context) []PendingProposal
	Payload(ctx context.Context, kind, scopeID, recordType, recordID string) ([]byte, bool, error)
	People(ctx context.Context) (map[string]PersonLabel, error)
	SelfApprovalPermitted(ctx context.Context, humanID, recordType string, now time.Time) bool
	CurrentSeverity(ctx context.Context, incidentID string) string
}

// ApprovalItem is one pending change as this person sees it.
type ApprovalItem struct {
	Group, Kind, Label, IncidentID, IncidentTitle, Compartment string
	Summary, Consequence, RejectConsequence, Reason            string
	Effective, Proposed                                        []Field
	Maker, MakerName, MakerRole                                string
	MakerExternal                                              bool
	ProposedAt                                                 time.Time
	ScopeID, RecordType, RecordID, Version                     string
	CanDecide, CanSelfApprove                                  bool
}

// ApprovalQueue is the Approvals screen's content.
type ApprovalQueue struct {
	Incidents              []IncidentRef
	Waiting, AwaitingOther []ApprovalItem
}

// ApprovalsService builds the approvals queue.
type ApprovalsService struct{ Backend ApprovalsBackend }

// selfApprovableKinds are the kinds a single-person window may cover.
var selfApprovableKinds = map[string]bool{"incident": true, "severity": true, "communication": true, "obligation": true, "recovery": true}

// Queue returns the pending changes this person may see, split into those
// they may decide and those waiting for someone else. filter limits it to one
// incident.
func (s ApprovalsService) Queue(ctx context.Context, principal authz.Principal, filter string, now time.Time) (ApprovalQueue, error) {
	if s.Backend == nil {
		return ApprovalQueue{}, fmt.Errorf("approvals backend is required")
	}
	if !principal.Authenticated || principal.HumanID == "" {
		return ApprovalQueue{}, ErrAuthorisationDenied
	}
	var sources []PendingProposal
	seen := map[string]bool{}
	add := func(p PendingProposal) {
		key := p.Kind + "|" + p.ScopeID + "|" + p.RecordType + "|" + p.RecordID
		if !seen[key] {
			seen[key] = true
			sources = append(sources, p)
		}
	}
	readable := func(incidentID, compartment string) bool {
		if compartment == "" {
			compartment = "GENERAL"
		}
		return authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}).Allowed
	}
	titles := map[string]string{}
	var out ApprovalQueue
	ids := make([]string, 0, len(principal.Incidents))
	for id, member := range principal.Incidents {
		if member {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		ref, ok := s.Backend.Incident(ctx, principal, id)
		if !ok {
			continue
		}
		titles[id] = ref.Title
		out.Incidents = append(out.Incidents, ref)
		if filter != "" && filter != id {
			continue
		}
		for _, compartment := range readableCompartmentsOf(principal, id) {
			for _, p := range s.Backend.PendingInIncident(ctx, principal, id, compartment) {
				if readable(id, p.Compartment) {
					add(p)
				}
			}
		}
		for _, p := range s.Backend.PendingRelationships(ctx, id) {
			// Shown only to people who can already see both incidents.
			if readable(p.RelatedIncidentID, "GENERAL") {
				add(p)
			}
		}
		for _, p := range s.Backend.PendingAccessForIncident(ctx, id) {
			if readable(id, p.Compartment) {
				add(p)
			}
		}
	}
	if filter == "" {
		// Organisation-wide changes are decided by Majors.
		if principal.Role == authz.RoleMajor {
			declarations, declared := s.Backend.PendingDeclarations(ctx)
			for id, title := range declared {
				titles[id] = title
			}
			for _, p := range declarations {
				add(p)
			}
			for _, p := range s.Backend.PendingSystemObligations(ctx) {
				add(p)
			}
			for _, p := range s.Backend.PendingOrganisation(ctx, principal) {
				add(p)
			}
		}
		if principal.Custodian {
			for _, p := range s.Backend.PendingAccessAdministration(ctx) {
				add(p)
			}
		}
	}
	people, err := s.Backend.People(ctx)
	if err != nil {
		return ApprovalQueue{}, err
	}
	for _, p := range sources {
		item := s.item(ctx, p, titles, people)
		item.CanDecide = p.Maker != principal.HumanID
		if !item.CanDecide && principal.Role == authz.RoleMajor && selfApprovableKinds[p.Kind] {
			item.CanSelfApprove = s.Backend.SelfApprovalPermitted(ctx, principal.HumanID, p.RecordType, now)
		}
		if item.CanDecide {
			out.Waiting = append(out.Waiting, item)
		} else {
			out.AwaitingOther = append(out.AwaitingOther, item)
		}
	}
	return out, nil
}

// readableCompartmentsOf lists GENERAL then the restricted compartments the
// person is read into, in name order.
func readableCompartmentsOf(principal authz.Principal, incidentID string) []string {
	out := []string{"GENERAL"}
	var restricted []string
	for name, allowed := range principal.Compartments[incidentID] {
		if allowed && name != "GENERAL" {
			restricted = append(restricted, name)
		}
	}
	sort.Strings(restricted)
	return append(out, restricted...)
}

func (s ApprovalsService) item(ctx context.Context, p PendingProposal, titles map[string]string, people map[string]PersonLabel) ApprovalItem {
	maker := people[p.Maker]
	item := ApprovalItem{
		Kind: p.Kind, Label: ApprovalLabel(p.RecordType), Compartment: p.Compartment, Maker: p.Maker,
		MakerName: maker.DisplayName, MakerRole: maker.Role, MakerExternal: maker.External, ProposedAt: p.ProposedAt,
		ScopeID: p.ScopeID, RecordType: p.RecordType, RecordID: p.RecordID, Version: p.Version,
	}
	if p.Kind != "obligation" || p.ScopeID != "" {
		item.IncidentID = p.ScopeID
		item.IncidentTitle = titles[p.ScopeID]
	}
	payload, found, err := s.Backend.Payload(ctx, p.Kind, p.ScopeID, p.RecordType, p.RecordID)
	if p.Kind == "organisation" {
		if p.Compartment == "SYSTEM" {
			item.Compartment = ""
		}
		if err == nil && found {
			item.Summary, item.Consequence, item.Proposed = organisationSummary(p.RecordType, payload)
		}
		item.RejectConsequence = rejectConsequence(p, nil)
		return item
	}
	values := map[string]any{}
	if err == nil && found {
		item.Proposed, values = payloadFields(payload)
	}
	item.Reason = firstString(values, "reason_evidence", "reason", "rationale", "applicability_reason")
	if template := firstString(values, "checklist_template_id"); template != "" {
		item.Group = "Checklist " + template + " · " + p.ScopeID
	}
	if strings.HasPrefix(p.RecordID, "PERSON-") {
		item.Group = "New person · " + firstString(values, "human_id")
	}
	if summary, consequence, ok := accessInOperatorLanguage(p, values, people); ok {
		item.Summary, item.Consequence = summary, consequence
	} else {
		item.Summary, item.Consequence = approvalSummary(p, values, item.Label)
	}
	item.RejectConsequence = rejectConsequence(p, values)
	if p.Kind == "severity" && item.IncidentID != "" {
		if current := s.Backend.CurrentSeverity(ctx, item.IncidentID); current != "" {
			item.Effective = []Field{{Name: "Severity", Value: current}}
		}
	}
	return item
}

// personPhrase names a person for an approval: "Jane Doe (EXTERNAL, Forensic
// Co, Minor)". Unknown people are named by their ID.
func personPhrase(id string, people map[string]PersonLabel) string {
	p, ok := people[id]
	if !ok || p.DisplayName == "" {
		return id
	}
	var parts []string
	if p.External {
		parts = append(parts, "EXTERNAL")
	}
	if p.Organisation != "" {
		parts = append(parts, p.Organisation)
	}
	if p.Role != "" {
		parts = append(parts, p.Role)
	}
	if len(parts) == 0 {
		return p.DisplayName
	}
	return p.DisplayName + " (" + strings.Join(parts, ", ") + ")"
}

// accessInOperatorLanguage words incident access, read-in and role changes as
// an operator would say them (R-22).
func accessInOperatorLanguage(p PendingProposal, v map[string]any, people map[string]PersonLabel) (string, string, bool) {
	str := func(k string) string { return firstString(v, k) }
	who := personPhrase(str("human_id"), people)
	active, _ := v["active"].(bool)
	until := ""
	if exp := str("expires_at"); exp != "" {
		if t, err := time.Parse(time.RFC3339Nano, exp); err == nil {
			until = " until " + t.UTC().Format(DisplayTime)
		}
	}
	switch p.RecordType {
	case access.RecordIncidentMembership:
		if !active {
			return "Remove " + who + " from " + p.ScopeID, "They lose access to " + p.ScopeID + " on every device at once. What they recorded is kept.", true
		}
		consequence := "They can see and work in " + p.ScopeID + " (GENERAL) from their next request."
		if until != "" {
			consequence += " Access ends automatically" + until + "."
		}
		return "Give " + who + " access to " + p.ScopeID + until, consequence, true
	case access.RecordCompartmentMembership:
		c := str("compartment")
		if !active {
			return "Remove " + who + " from " + c + " on " + p.ScopeID, "They stop seeing " + c + " records at once. What they recorded is kept.", true
		}
		consequence := "They can see " + c + " records in " + p.ScopeID + "."
		if until != "" {
			consequence += " Read-in ends automatically" + until + "."
		}
		return "Read " + who + " into " + c + " on " + p.ScopeID + until, consequence, true
	case access.RecordRoleEligibility:
		if !active {
			return "Remove the CRAB role of " + who, "They keep their identity but can no longer act in any incident.", true
		}
		role := str("role")
		return "Make " + who + " eligible as " + role, role + " lets them work in incidents they are later given access to. It grants no incident access by itself.", true
	case access.RecordIncidentSponsor:
		if !active {
			return "Remove " + who + " as sponsor of " + p.ScopeID, "They can no longer propose access to " + p.ScopeID + ".", true
		}
		return "Make " + who + " sponsor of " + p.ScopeID, "They can propose who gets access to " + p.ScopeID + ". A Custodian still confirms each change.", true
	case access.RecordCompartmentController:
		c := str("compartment")
		if !active {
			return "Remove " + who + " as controller of " + c + " on " + p.ScopeID, "They can no longer propose read-ins to " + c + ".", true
		}
		return "Make " + who + " controller of " + c + " on " + p.ScopeID, "They can propose who is read into " + c + ". A second person still confirms each read-in.", true
	}
	return "", "", false
}

// rejectConsequence says what happens if the proposal is rejected (R-22).
func rejectConsequence(p PendingProposal, v map[string]any) string {
	base := " The proposal is kept in history as rejected; it can be proposed again."
	switch p.RecordType {
	case domain.TypeIncidentDeclaration:
		return "No incident is declared." + base
	case domain.TypeClosure:
		return "The incident stays open and writable." + base
	case domain.TypeReopening:
		return "The incident stays closed and read-only." + base
	case domain.TypeSeverityChange:
		return "Severity stays as it is." + base
	case domain.TypeCommunicationIssue:
		return "Nothing is sent. The draft stays a draft." + base
	case access.RecordCredentialReset:
		return "No code is issued; their current sign-in is unchanged." + base
	case access.RecordIntakeKey:
		return "The intake key is unchanged." + base
	case access.RecordIdentity:
		return "The person's account stays as it is." + base
	case access.RecordIncidentMembership, access.RecordCompartmentMembership:
		return "Their access stays as it is." + base
	}
	return "Nothing changes." + base
}

// ApprovalLabel names a record type for people.
func ApprovalLabel(recordType string) string {
	labels := map[string]string{
		domain.TypeIncidentDeclaration:       "Incident declaration",
		domain.TypeDecision:                  "Decision",
		domain.TypeAction:                    "Action",
		domain.TypeOperationalRoleAssignment: "Operational role",
		domain.TypeSeverityChange:            "Severity change",
		domain.TypeCommunicationIssue:        "Communication issue",
		domain.TypeCommunicationCadence:      "Report cadence",
		domain.TypeObligationDefinition:      "Obligation definition",
		domain.TypeObligationTrigger:         "Obligation clock",
		domain.TypeObligationChange:          "Obligation change",
		domain.TypeStandDown:                 "Stand-down",
		domain.TypePIR:                       "Post-Incident Review",
		domain.TypeRetention:                 "Retention and hold",
		domain.TypeClosure:                   "Incident closure",
		domain.TypeReopening:                 "Reopen incident",
		"security.incident_membership":       "Incident access",
		"security.compartment_membership":    "Compartment read-in",
		"security.incident_sponsor":          "Incident sponsor",
		"security.compartment_controller":    "Compartment controller",
		"security.isolate_mode":              "Isolate mode",
		"security.output_permission":         "Print or download permission",
		domain.TypeSeverityScale:             "Severity scale",
		domain.TypeIncidentRelationship:      "Related incident",
		domain.TypeEmergencyConfiguration:    "Emergency information",
		domain.TypeChecklistTemplate:         "Checklist template",
		domain.TypeSinglePersonWindow:        "Single-person window",
		"security.identity":                  "Person",
		"security.role_eligibility":          "Role",
		"security.credential_reset":          "Enrolment code",
		"security.intake_key":                "Intake key",
		"security.custodian_designation":     "Custodian",
	}
	if label, ok := labels[recordType]; ok {
		return label
	}
	name := recordType
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	name = strings.ReplaceAll(name, "_", " ")
	if name == "" {
		return "Change"
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func approvalSummary(s PendingProposal, v map[string]any, label string) (string, string) {
	if summary, consequence, ok := AccessApprovalSummary(s.RecordType, v); ok {
		return summary, consequence
	}
	if summary, consequence, ok := RecoveryApprovalSummary(s.RecordType, v); ok {
		return summary, consequence
	}
	str := func(keys ...string) string { return firstString(v, keys...) }
	switch s.RecordType {
	case domain.TypeIncidentDeclaration:
		return "Declare incident: " + str("title"), "The incident becomes active and its declared severity takes effect."
	case domain.TypeSeverityChange:
		return "Change severity to " + str("severity"), "Severity becomes " + str("severity") + " on the Board, brief and handover."
	case domain.TypeDecision:
		return str("decision"), "The decision is recorded as confirmed."
	case domain.TypeAction:
		status := str("status")
		if status != "" && status != domain.ActionStatusNotStarted {
			return "Action " + strings.ReplaceAll(status, "_", " ") + ": " + str("title"), ""
		}
		return "New action: " + str("title") + " (owner " + str("owner_human_id") + ")", ""
	case domain.TypeOperationalRoleAssignment:
		who := str("human_id")
		if b, _ := v["deputy"].(bool); b {
			return "Make " + who + " deputy " + str("role_name"), ""
		}
		return "Make " + who + " " + str("role_name"), "The previous holder's history is kept."
	case domain.TypeCommunicationIssue:
		return "Issue: " + str("subject"), "The approved wording may then be sent on its channels. Exercise notifications are suppressed."
	case domain.TypeCommunicationCadence:
		return "Report cadence: " + str("name"), ""
	case domain.TypeObligationTrigger:
		return "Start obligation clock " + str("definition_id"), "The provisional clock becomes firm."
	case domain.TypeObligationChange:
		return "Change obligation clock " + str("clock_id") + " (" + str("kind") + ")", "The original deadline and rule are preserved."
	case domain.TypeObligationDefinition:
		return "Obligation: " + str("name"), "Becomes available to start clocks in every incident."
	case domain.TypeIncidentRelationship:
		if b, _ := v["active"].(bool); !b {
			return "Remove link between " + s.ScopeID + " and " + str("related_incident_id"), "The link is withdrawn. History is kept."
		}
		return "Relate " + s.ScopeID + " to " + str("related_incident_id"), "Each incident shows the link to people who can see both. Nobody gains access, and nothing is merged."
	case domain.TypeStandDown:
		return "Stand down the emergency response", "The emergency phase ends. The incident stays open for review."
	case domain.TypePIR:
		return "Confirm the Post-Incident Review", "Closure becomes possible once other prerequisites are met."
	case domain.TypeRetention:
		return "Retention: " + str("retention_class"), ""
	case domain.TypeClosure:
		return "Close the incident", "The incident becomes read-only. Privileged and shareable closure records are produced."
	case "security.incident_membership", "security.compartment_membership":
		verb := "Grant"
		if b, ok := v["active"].(bool); ok && !b {
			verb = "Revoke"
		}
		target := "incident access"
		if s.RecordType == "security.compartment_membership" {
			target = str("compartment") + " read-in"
		}
		consequence := ""
		if exp := str("expires_at"); exp != "" && verb == "Grant" {
			consequence = "Access ends automatically at " + exp + "."
		}
		return verb + " " + target + " for " + str("human_id"), consequence
	}
	return label, ""
}

func firstString(v map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := v[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// payloadFields turns a proposal payload into labelled fields in stored order.
func payloadFields(payload []byte) ([]Field, map[string]any) {
	values := map[string]any{}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	var fields []Field
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, values
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			break
		}
		key, _ := keyTok.(string)
		var raw any
		if err := dec.Decode(&raw); err != nil {
			break
		}
		values[key] = raw
		if text := fieldText(raw); text != "" {
			fields = append(fields, Field{Name: fieldName(key), Value: text})
		}
	}
	return fields, values
}

func fieldName(key string) string {
	names := map[string]string{
		"owner_human_id": "Owner", "human_id": "Person", "role_name": "Role", "reason_evidence": "Reason and evidence",
		"assigned_to_human_id": "Assigned to", "source_ref": "Source", "due_at": "Due", "interval_minutes": "Every (minutes)",
		"first_due_at": "First due", "draft_sha256": "Approved wording (SHA-256)", "draft_record_id": "Draft",
		"definition_id": "Obligation", "trigger_at": "Trigger time", "source_record_id": "Source record",
		"new_deadline": "New deadline", "expires_at": "Expires", "effective_at": "Effective from",
	}
	if name, ok := names[key]; ok {
		return name
	}
	key = strings.ReplaceAll(key, "_", " ")
	if key == "" {
		return ""
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

// machineWord matches stored enum values such as not_started.
var machineWord = regexp.MustCompile(`^[a-z]+(_[a-z]+)+$`)

func fieldText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
			return parsed.UTC().Format(DisplayTime)
		}
		if machineWord.MatchString(t) {
			return strings.ReplaceAll(t, "_", " ")
		}
		return t
	case bool:
		if t {
			return "Yes"
		}
		return "No"
	case json.Number:
		return t.String()
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s := fieldText(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(b)
	}
}

// organisationSummary describes an organisation proposal in operator words.
func organisationSummary(recordType string, payload []byte) (string, string, []Field) {
	switch recordType {
	case domain.TypeSeverityScale:
		var v domain.SeverityScale
		_ = json.Unmarshal(payload, &v)
		return "New severity scale", "Responders choose only from these levels. Existing incidents keep their severity until changed.", []Field{{Name: "Levels, least severe first", Value: joinLabels(v.Labels())}}
	case domain.TypeEmergencyConfiguration:
		var v domain.EmergencyConfiguration
		_ = json.Unmarshal(payload, &v)
		fields := []Field{
			{Name: "Organisation", Value: v.Organisation},
			{Name: "Emergency contacts", Value: countOf(len(v.Contacts))},
			{Name: "Insurers, regulators and advisers", Value: countOf(len(v.Parties))},
			{Name: "Escalation steps", Value: countOf(len(v.Escalation))},
			{Name: "Role cards", Value: countOf(len(v.RoleCards))},
			{Name: "Procedures", Value: countOf(len(v.Procedures))},
		}
		for _, c := range v.Contacts {
			fields = append(fields, Field{Name: c.Role, Value: c.Name + " · " + c.Primary})
		}
		return "Emergency information for " + v.Organisation, "The next Emergency Response Pack and every Incident Brief print this information. The current pack becomes stale.", fields
	}
	if recordType == domain.TypeSinglePersonWindow {
		var v domain.SinglePersonWindow
		_ = json.Unmarshal(payload, &v)
		if !v.Active {
			return "Close single-person window " + v.WindowID, "Self-approval stops immediately.", []Field{{Name: "Reason", Value: v.Reason}}
		}
		kinds := make([]string, 0, len(v.Kinds))
		for _, k := range v.Kinds {
			kinds = append(kinds, domain.WindowCoverableKinds[k])
		}
		return "Open single-person window " + v.WindowID, "Between these times the named Majors may decide their own proposals of these kinds, with a reason. Each is flagged SELF-APPROVAL.", []Field{
			{Name: "From (UTC)", Value: v.StartsAt.UTC().Format(DisplayTime)},
			{Name: "To (UTC)", Value: v.EndsAt.UTC().Format(DisplayTime)},
			{Name: "Who", Value: joinWith(v.Humans, ", ")},
			{Name: "Covers", Value: joinWith(kinds, ", ")},
			{Name: "Reason", Value: v.Reason},
		}
	}
	if recordType == domain.TypeChecklistTemplate {
		var v domain.GovernedChecklistTemplate
		_ = json.Unmarshal(payload, &v)
		if !v.Active {
			return "Withdraw checklist: " + v.Name, "Responders can no longer activate it. Actions already created are unchanged.", nil
		}
		fields := []Field{{Name: "Source", Value: v.SourceRef}}
		for i, step := range v.Steps {
			detail := step.Priority
			if step.DefaultRole != "" {
				detail += " · " + step.DefaultRole
			}
			fields = append(fields, Field{Name: "Step " + strconv.Itoa(i+1), Value: step.Title + " (" + detail + ")"})
		}
		return "Checklist template: " + v.Name, "Responders can activate it on any incident; each step becomes an action that still needs a second person.", fields
	}
	return "Organisation change", "", nil
}

func joinLabels(labels []string) string {
	out := ""
	for i, l := range labels {
		if i > 0 {
			out += " → "
		}
		out += l
	}
	return out
}

func countOf(n int) string {
	if n == 0 {
		return "none"
	}
	return strconv.Itoa(n)
}

func joinWith(values []string, sep string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += sep
		}
		out += v
	}
	return out
}
