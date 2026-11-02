package app

import (
	"strings"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// T-244 (R-19): after a suspend and resume, CRAB has no queue of timers to
// replay: deadlines are judged when read, so each clock that passed while
// suspended shows once as overdue, stating its original deadline (late), and
// nothing fires twice.
func TestResumeShowsEachMissedDeadlineOnceAsLate(t *testing.T) {
	suspended := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	clocks := []domain.ObligationClockView{
		{ClockID: "C1", Name: "Regulator notice", Status: domain.ObligationStatusActive, Deadline: suspended.Add(2 * time.Hour)},
		{ClockID: "C2", Name: "Insurer notice", Status: domain.ObligationStatusActive, Deadline: suspended.Add(5 * time.Hour)},
		{ClockID: "C3", Name: "Customer notice", Status: domain.ObligationStatusActive, Deadline: suspended.Add(48 * time.Hour)},
	}
	resumed := suspended.Add(9 * time.Hour)
	for i := 0; i < 2; i++ { // reading twice changes nothing
		now := strings.Join(texts(BoardNow(BoardFocusInput{TimeStatus: ledger.TimeTrusted, Clocks: clocks, Now: resumed})), "\n")
		for _, want := range []string{"Overdue: Regulator notice — deadline was 2 Nov 2026 03:00 UTC", "Overdue: Insurer notice — deadline was 2 Nov 2026 06:00 UTC", "Next deadline: Customer notice"} {
			if strings.Count(now, want) != 1 {
				t.Fatalf("read %d: want %q exactly once in:\n%s", i+1, want, now)
			}
		}
	}
}

func texts(items []NowItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Text
	}
	return out
}

// T-244: a 72-hour obligation triggered just before the UK clocks go forward
// is due exactly 72 elapsed hours later; times are UTC throughout.
func TestElapsedDeadlineSurvivesUKDaylightSaving(t *testing.T) {
	def := domain.ObligationDefinition{Name: "Regulator notification", TriggerField: "awareness", Duration: 72, Unit: domain.ObligationUnitElapsedHours,
		Calendar: domain.BusinessCalendar{Timezone: "Europe/London", Adjustment: "none"}, ApplicabilityCondition: "Personal data", Recipient: "Regulator", Owner: "legal", AuthoritySource: "Statute"}
	trigger := time.Date(2026, 3, 28, 23, 30, 0, 0, time.UTC) // clocks go forward 29 Mar 01:00 UTC
	d, err := domain.CalculateObligationDeadline(def, trigger)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Deadline.Sub(trigger); got != 72*time.Hour {
		t.Fatalf("deadline is %s after the trigger, want 72h", got)
	}
}
