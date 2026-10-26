package pdf

import (
	"os"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

// TestRenderSamples writes representative PDFs for human print review (T-212)
// when CRAB_PDF_SAMPLE_DIR is set; it is skipped otherwise.
func TestRenderSamples(t *testing.T) {
	out := os.Getenv("CRAB_PDF_SAMPLE_DIR")
	if out == "" {
		t.Skip()
	}
	due := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	brief := domain.IncidentBrief{Source: testSource(), Compartment: "GENERAL",
		Incident:                  domain.IncidentState{IncidentID: "INC-014", Mode: "live", Title: "Payment outage after suspected ransomware on payroll servers", Summary: "Payroll and payments services are unavailable. Three file servers show encryption. Backups are being verified. Customers cannot make card payments through the web portal.", Severity: "Severity level 4"},
		Facts:                     []domain.FactView{{Fact: domain.Fact{Statement: "Payroll file share FS-02 encrypted at 07:42 UTC", Source: "EDR console"}}, {Fact: domain.Fact{Statement: "Ransom note references group 'Bafflewick'"}}},
		Unknowns:                  []domain.UncertaintyView{{Uncertainty: domain.Uncertainty{Kind: "unknown", Statement: "Initial access vector"}}},
		Hypotheses:                []domain.UncertaintyView{{Uncertainty: domain.Uncertainty{Kind: "hypothesis", Statement: "Compromised supplier VPN account"}}},
		Decisions:                 []domain.DecisionView{{Decision: domain.Decision{Decision: "Isolate payroll VLAN from corporate network"}}},
		Actions:                   []domain.ActionView{{RecordID: "A1", Action: domain.Action{Title: "Verify last good backup of FS-02 and confirm restore point", OwnerHumanID: "carol", Priority: "P1", Status: "in_progress", DueAt: &due}}, {RecordID: "A2", Action: domain.Action{Title: "Notify cyber insurer hotline", OwnerHumanID: "dave", Priority: "P2", Status: "open"}}},
		Obligations:               []domain.ObligationClockView{{Name: "ICO personal data breach", Deadline: due.Add(48 * time.Hour), Lifecycle: "provisional", Status: domain.ObligationStatusActive, Owner: "legal", TimeStatus: "untrusted"}},
		OperationalRoles:          []domain.OperationalRoleView{{Assignment: domain.OperationalRoleAssignment{RoleName: "Incident commander", HumanID: "alice"}}, {Assignment: domain.OperationalRoleAssignment{RoleName: "Technical lead", HumanID: "bob", Deputy: true}}},
		NextCommunicationDeadline: &due,
		Contacts:                  []domain.PrintContact{{Role: "Insurer", Name: "Example Mutual cyber line", Primary: "+44 20 7946 0000", Alternate: "claims@example.org"}},
	}
	for _, mode := range []string{domain.PrintColour, domain.PrintMonochrome} {
		b, _, err := Renderer{}.RenderIncidentBrief(brief, domain.PageSizeA4, mode)
		if err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(out+"/brief-"+mode+".pdf", b, 0o644)
	}
	pack := domain.EmergencyResponsePack{Source: testSource(), Organisation: "Bafflewick Ltd", Mode: "exercise", Sections: []domain.PackSection{
		{Title: "Emergency contacts", Columns: []string{"Role", "Name", "Primary", "Alternative"}, Widths: []float64{0.25, 0.25, 0.25, 0.25}, Rows: [][]string{{"Incident commander", "Alice Smith", "+44 7700 900001", "+44 20 7946 0001"}}},
		{Title: "Operating rule", Body: "The model proposes. One person makes. Another confirms. Nothing is erased."}},
		Forms: domain.DefaultContingencyForms()}
	b, _, err := Renderer{}.RenderEmergencyResponsePack(pack, domain.PageSizeLetter, domain.PrintColour)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(out+"/pack.pdf", b, 0o644)
}
