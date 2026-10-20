package domain

import (
	"testing"
	"time"
)

func testObligationDefinition() ObligationDefinition {
	return ObligationDefinition{
		Name:                   "Regulator notification",
		TriggerField:           "flagged_at",
		Duration:               3,
		Unit:                   ObligationUnitBusinessDays,
		Calendar:               BusinessCalendar{Timezone: "Australia/Sydney", Weekend: []string{"Saturday", "Sunday"}, Holidays: []string{"2026-10-26"}, Adjustment: "next_business_day"},
		ApplicabilityCondition: "Organisation has determined the configured regulator rule applies",
		Recipient:              "Configured regulator",
		Owner:                  "Legal lead",
		AuthoritySource:        "Regulatory response plan v4.2",
		WarningBeforeMinutes:   []int{1440, 120},
	}
}

func TestObligationClockBusinessDaysAndHoliday(t *testing.T) {
	def := testObligationDefinition()
	loc, err := time.LoadLocation("Australia/Sydney")
	if err != nil {
		t.Fatal(err)
	}
	trigger := time.Date(2026, 10, 23, 10, 0, 0, 0, loc) // Friday; Monday 26th is configured holiday.
	result, err := CalculateObligationDeadline(def, trigger)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 29, 10, 0, 0, 0, loc)
	if !result.Deadline.Equal(want) {
		t.Fatalf("deadline=%v want=%v", result.Deadline, want)
	}
}

func TestObligationClockElapsedHoursCrossesDSTAsElapsedTime(t *testing.T) {
	def := testObligationDefinition()
	def.Duration = 24
	def.Unit = ObligationUnitElapsedHours
	def.Calendar.Holidays = nil
	loc, err := time.LoadLocation("Australia/Sydney")
	if err != nil {
		t.Fatal(err)
	}
	trigger := time.Date(2026, 10, 4, 1, 30, 0, 0, loc) // DST starts at 02:00 local.
	result, err := CalculateObligationDeadline(def, trigger)
	if err != nil {
		t.Fatal(err)
	}
	if result.Deadline.Sub(trigger) != 24*time.Hour {
		t.Fatalf("elapsed duration=%v", result.Deadline.Sub(trigger))
	}
	if result.Deadline.Hour() != 2 || result.Deadline.Day() != 5 {
		t.Fatalf("elapsed deadline local=%v", result.Deadline)
	}
}

func TestObligationClockCalendarDayRejectsNonexistentWallTime(t *testing.T) {
	def := testObligationDefinition()
	def.Duration = 1
	def.Unit = ObligationUnitCalendarDays
	def.Calendar.Holidays = nil
	loc, err := time.LoadLocation("Australia/Sydney")
	if err != nil {
		t.Fatal(err)
	}
	trigger := time.Date(2026, 10, 3, 2, 30, 0, 0, loc)
	if _, err := CalculateObligationDeadline(def, trigger); err == nil {
		t.Fatal("calendar arithmetic silently coerced nonexistent DST wall time")
	}
}

func TestObligationClockRefusesTimezoneMismatchAndContradictoryCalendar(t *testing.T) {
	def := testObligationDefinition()
	if _, err := CalculateObligationDeadline(def, time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("timezone/offset contradiction accepted")
	}
	def.Calendar.Weekend = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	if err := ValidateObligationDefinition(def); err == nil {
		t.Fatal("calendar with no business weekday accepted")
	}
}

func TestObligationDefinitionIsGovernedSystemProposal(t *testing.T) {
	proposal, err := ProposeObligationDefinition("OBL-DEF-1", "alice", time.Now().UTC(), testObligationDefinition())
	if err != nil {
		t.Fatal(err)
	}
	if proposal.IncidentMode != "system" || proposal.IncidentID != "" || proposal.WriteSemantic != "governed" || proposal.Status != "proposed" {
		t.Fatalf("definition proposal=%+v", proposal)
	}
	version, err := ProposalVersion(proposal)
	if err != nil || version == "" {
		t.Fatalf("proposal version=%q err=%v", version, err)
	}
}

func TestObligationTriggerAndChangeRequireExplicitHumanContext(t *testing.T) {
	scope := Scope{IncidentID: "INC-1", Mode: "live", Compartment: "GENERAL"}
	_, err := ProposeObligationTrigger(scope, "CLOCK-1", "alice", time.Now().UTC(), ObligationTrigger{
		DefinitionID: "OBL-DEF-1", SourceRecordID: "FACT-1", SourceField: "flagged_at", TriggerAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("trigger without applicability reason accepted")
	}
	deadline := time.Now().UTC().Add(24 * time.Hour)
	if _, err := ProposeObligationChange(scope, "CHANGE-1", "alice", time.Now().UTC(), ObligationChange{ClockID: "CLOCK-1", Kind: "extend", NewDeadline: &deadline}); err == nil {
		t.Fatal("change without authority/reason accepted")
	}
}

func TestObligationOverdueIsDerived(t *testing.T) {
	deadline := time.Date(2026, 10, 20, 3, 0, 0, 0, time.UTC)
	if ObligationOverdue(ObligationStatusWaived, deadline, deadline.Add(time.Hour)) {
		t.Fatal("waived obligation reported overdue")
	}
	if !ObligationOverdue(ObligationStatusActive, deadline, deadline.Add(time.Second)) {
		t.Fatal("active late obligation not reported overdue")
	}
}
