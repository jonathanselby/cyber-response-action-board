package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// Command Board focus (R-21, triage §3.15): the Now block and the order of
// each long section. The application decides; the template renders.

// NowLimit is the most items the Now block shows.
const NowLimit = 7

// BoardSectionLimit is how many items each long section shows before the
// rest are folded behind "All N …".
const BoardSectionLimit = 5

// NowItem is one line of the Now block. Anchor is a fragment on the Board or
// a path; Tone is danger, warning or info.
type NowItem struct {
	Text, Link, Tone string
}

// BoardFocusInput is what the Now block is built from, already scoped to what
// the viewer may see.
type BoardFocusInput struct {
	Viewer              string
	TimeStatus          string
	IntegrityFailed     bool
	Picture             domain.OperatingPicture
	Clocks              []domain.ObligationClockView
	FailedNotifications int
	LastHandoverAt      *time.Time
	Now                 time.Time
}

// IsCriticalPriority reports whether an action priority counts as critical
// (shared with the management summary).
func IsCriticalPriority(priority string) bool {
	p := strings.ToUpper(strings.TrimSpace(priority))
	for _, c := range domain.CriticalActionPriorities {
		if p == c {
			return true
		}
	}
	return false
}

// BoardNow returns at most NowLimit items in the order: integrity or time
// problems; overdue clocks, then the next clock due; overdue critical
// actions; approvals waiting for the viewer; failed notifications; unknowns
// recorded since the last handover.
func BoardNow(in BoardFocusInput) []NowItem {
	var out []NowItem
	add := func(text, link, tone string) {
		if len(out) < NowLimit {
			out = append(out, NowItem{Text: text, Link: link, Tone: tone})
		}
	}
	if in.IntegrityFailed {
		add("Ledger integrity check failed — records cannot be trusted until it is resolved", "/readiness", "danger")
	}
	if in.TimeStatus != "" && in.TimeStatus != ledger.TimeTrusted {
		add("Time is "+in.TimeStatus+" — times recorded now are marked unverified", "/readiness#time", "danger")
	}
	clocks := append([]domain.ObligationClockView(nil), in.Clocks...)
	sort.SliceStable(clocks, func(i, j int) bool { return clocks[i].Deadline.Before(clocks[j].Deadline) })
	nextShown := false
	for _, c := range clocks {
		if c.Status != domain.ObligationStatusActive {
			continue
		}
		if c.Overdue || in.Now.After(c.Deadline) {
			add("Overdue: "+c.Name+" — deadline was "+c.Deadline.UTC().Format(DisplayTime), "#clocks", "danger")
			continue
		}
		if !nextShown {
			nextShown = true
			add("Next deadline: "+c.Name+" — due "+c.Deadline.UTC().Format(DisplayTime)+" ("+untilText(c.Deadline.Sub(in.Now))+")", "#clocks", "warning")
		}
	}
	overdue := 0
	for _, a := range in.Picture.Actions {
		if IsCriticalPriority(a.Action.Priority) && domain.ActionOverdue(a.Action, in.Now) {
			overdue++
		}
	}
	if overdue > 0 {
		add(plural(overdue, "critical action is", "critical actions are")+" overdue", "#actions", "danger")
	}
	waiting := 0
	for _, p := range in.Picture.Pending {
		if p.Maker != in.Viewer {
			waiting++
		}
	}
	if waiting > 0 {
		add(plural(waiting, "approval is", "approvals are")+" waiting for you", "/approvals?incident="+in.Picture.Incident.IncidentID, "warning")
	}
	if in.FailedNotifications > 0 {
		add(plural(in.FailedNotifications, "notification", "notifications")+" failed in the last 24 hours", "#comms", "danger")
	}
	if in.LastHandoverAt != nil {
		fresh := 0
		for _, u := range in.Picture.Unknowns {
			if u.RecordedAt.After(*in.LastHandoverAt) {
				fresh++
			}
		}
		if fresh > 0 {
			add(plural(fresh, "new unknown", "new unknowns")+" since the last handover", "#knowledge", "info")
		}
	}
	return out
}

// OrderBoard puts each long section's most urgent items first: actions that
// are overdue, then open critical ones, then by due time; unknowns and
// decisions newest first; clocks by deadline.
func OrderBoard(p domain.OperatingPicture, clocks []domain.ObligationClockView, now time.Time) (domain.OperatingPicture, []domain.ObligationClockView) {
	actions := append([]domain.ActionView(nil), p.Actions...)
	rank := func(a domain.ActionView) int {
		open := a.Action.Status != domain.ActionStatusComplete && a.Action.Status != domain.ActionStatusSkipped
		switch {
		case open && domain.ActionOverdue(a.Action, now) && IsCriticalPriority(a.Action.Priority):
			return 0
		case open && domain.ActionOverdue(a.Action, now):
			return 1
		case open && IsCriticalPriority(a.Action.Priority):
			return 2
		case open:
			return 3
		}
		return 4
	}
	sort.SliceStable(actions, func(i, j int) bool {
		ri, rj := rank(actions[i]), rank(actions[j])
		if ri != rj {
			return ri < rj
		}
		di, dj := actions[i].Action.DueAt, actions[j].Action.DueAt
		switch {
		case di != nil && dj != nil:
			return di.Before(*dj)
		case di != nil:
			return true
		}
		return false
	})
	p.Actions = actions
	p.Unknowns = append([]domain.UncertaintyView(nil), p.Unknowns...)
	sort.SliceStable(p.Unknowns, func(i, j int) bool { return p.Unknowns[i].RecordedAt.After(p.Unknowns[j].RecordedAt) })
	p.Decisions = append([]domain.DecisionView(nil), p.Decisions...)
	sort.SliceStable(p.Decisions, func(i, j int) bool { return p.Decisions[i].ConfirmedAt.After(p.Decisions[j].ConfirmedAt) })
	ordered := append([]domain.ObligationClockView(nil), clocks...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Deadline.Before(ordered[j].Deadline) })
	return p, ordered
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func untilText(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh %02dm", int(d.Hours()), int(d.Minutes())%60)
}
