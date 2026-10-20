package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type fakePrintIncident struct {
	calls   int
	picture domain.OperatingPicture
}

func (f *fakePrintIncident) CommonOperatingPicture(context.Context, authz.Principal, string, string) (domain.OperatingPicture, bool, error) {
	f.calls++
	return f.picture, true, nil
}

type fakePrintSource struct {
	mark     domain.PrintSourceMark
	deadline *time.Time
}

func (f fakePrintSource) LedgerSource(context.Context, time.Time, string) (domain.PrintSourceMark, error) {
	return f.mark, nil
}
func (f fakePrintSource) NextCommunicationDeadline(context.Context, string, string) (*time.Time, error) {
	return f.deadline, nil
}

type fakePDFRenderer struct {
	brief domain.IncidentBrief
	pack  domain.EmergencyResponsePack
	err   error
}

func (f *fakePDFRenderer) RenderIncidentBrief(v domain.IncidentBrief, _, _ string) ([]byte, int, error) {
	f.brief = v
	if f.err != nil {
		return nil, 0, f.err
	}
	return []byte("pdf"), 1, nil
}
func (f *fakePDFRenderer) RenderEmergencyResponsePack(v domain.EmergencyResponsePack, _, _ string) ([]byte, int, error) {
	f.pack = v
	if f.err != nil {
		return nil, 0, f.err
	}
	return []byte("pdf"), 6, nil
}
func (f *fakePDFRenderer) RenderEvidenceRegister(domain.EvidenceRegisterDocument, string, string) ([]byte, int, error) {
	return []byte("pdf"), 1, f.err
}
func (f *fakePDFRenderer) RenderTimeline(domain.TimelineDocument, string, string) ([]byte, int, error) {
	return []byte("pdf"), 1, f.err
}
func (f *fakePDFRenderer) RenderReviewClosure(domain.ReviewClosureDocument, string, string) ([]byte, int, error) {
	return []byte("pdf"), 1, f.err
}

type fakeArtifactStore struct{ writes int }

func (f *fakeArtifactStore) WriteArtifact(string, []byte) (ArtifactWriteResult, error) {
	f.writes++
	return ArtifactWriteResult{PrimaryPath: "/tmp/derived.pdf"}, nil
}

func printPrincipal(withPrint bool) authz.Principal {
	outputs := map[authz.Action]bool{}
	if withPrint {
		outputs[authz.ActionPrint] = true
	}
	return authz.Principal{
		Authenticated: true,
		HumanID:       "alice",
		Role:          authz.RoleMajor,
		Incidents:     map[string]bool{"INC-PRINT": true},
		Compartments:  map[string]map[string]bool{"INC-PRINT": {"GENERAL": true}},
		Outputs:       outputs,
	}
}

func printService(incident *fakePrintIncident, renderer *fakePDFRenderer, artifacts *fakeArtifactStore, mark domain.PrintSourceMark) PrintService {
	return PrintService{Incident: incident, Source: fakePrintSource{mark: mark}, Renderer: renderer, Artifacts: artifacts, ConfigurationVersion: "cfg-11"}
}

func TestPrintPermissionIsSeparateAndDeniedBeforeIncidentRead(t *testing.T) {
	incident := &fakePrintIncident{picture: domain.OperatingPicture{Incident: domain.IncidentState{IncidentID: "INC-PRINT", Mode: "live"}}}
	service := printService(incident, &fakePDFRenderer{}, &fakeArtifactStore{}, printTestSource())
	_, err := service.GenerateIncidentBrief(context.Background(), printPrincipal(false), "INC-PRINT", "GENERAL", nil, domain.PageSizeA4, domain.PrintMonochrome, time.Now())
	if !errors.Is(err, ErrAuthorisationDenied) {
		t.Fatalf("print without output permission err=%v", err)
	}
	if incident.calls != 0 {
		t.Fatalf("incident content read before print authorisation: %d calls", incident.calls)
	}
}

func TestRestrictedCompartmentPrintRequiresReadIn(t *testing.T) {
	incident := &fakePrintIncident{}
	service := printService(incident, &fakePDFRenderer{}, &fakeArtifactStore{}, printTestSource())
	_, err := service.GenerateIncidentBrief(context.Background(), printPrincipal(true), "INC-PRINT", "LEGAL", nil, domain.PageSizeA4, domain.PrintMonochrome, time.Now())
	if !errors.Is(err, ErrAuthorisationDenied) || incident.calls != 0 {
		t.Fatalf("restricted print err=%v calls=%d", err, incident.calls)
	}
}

func TestIncidentBriefComposesCurrentStateAndCRABSourceMark(t *testing.T) {
	deadline := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	incident := &fakePrintIncident{picture: domain.OperatingPicture{
		Incident: domain.IncidentState{IncidentID: "INC-PRINT", Mode: "live", Title: "Bafflewick", Summary: "Current situation", Severity: "High"},
		Facts:    []domain.FactView{{RecordID: "FACT-1", Fact: domain.Fact{Statement: "Current fact"}}},
	}}
	renderer := &fakePDFRenderer{}
	artifacts := &fakeArtifactStore{}
	mark := printTestSource()
	service := PrintService{Incident: incident, Source: fakePrintSource{mark: mark, deadline: &deadline}, Renderer: renderer, Artifacts: artifacts, ConfigurationVersion: "cfg-11"}
	result, err := service.GenerateIncidentBrief(context.Background(), printPrincipal(true), "INC-PRINT", "GENERAL", nil, domain.PageSizeA4, domain.PrintMonochrome, mark.GeneratedAt)
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.writes != 1 || renderer.brief.Source != mark || renderer.brief.Incident.Summary != "Current situation" || renderer.brief.NextCommunicationDeadline == nil || !renderer.brief.NextCommunicationDeadline.Equal(deadline) {
		t.Fatalf("result=%+v brief=%+v writes=%d", result, renderer.brief, artifacts.writes)
	}
}

func TestEmergencyPackSourceCannotBeSuppliedByCaller(t *testing.T) {
	incident := &fakePrintIncident{}
	renderer := &fakePDFRenderer{}
	artifacts := &fakeArtifactStore{}
	mark := printTestSource()
	service := printService(incident, renderer, artifacts, mark)
	callerMark := mark
	callerMark.LedgerHash = "forged"
	pack := domain.EmergencyResponsePack{Source: callerMark, Organisation: "Example Org", Mode: "live", Sections: []domain.PackSection{{Title: "Runbook", Body: "Approved response steps"}}, Forms: domain.DefaultContingencyForms()}
	result, err := service.GenerateEmergencyResponsePack(context.Background(), printPrincipal(true), pack, domain.PageSizeA4, domain.PrintMonochrome, mark.GeneratedAt)
	if err != nil {
		t.Fatal(err)
	}
	if renderer.pack.Source != mark || result.Source != mark {
		t.Fatalf("caller supplied source survived: rendered=%+v result=%+v", renderer.pack.Source, result.Source)
	}
}

func TestRendererFailureDoesNotWriteDerivedCopy(t *testing.T) {
	incident := &fakePrintIncident{picture: domain.OperatingPicture{Incident: domain.IncidentState{IncidentID: "INC-PRINT", Mode: "live", Title: "Incident", Summary: "Summary", Severity: "High"}}}
	renderer := &fakePDFRenderer{err: errors.New("renderer unavailable")}
	artifacts := &fakeArtifactStore{}
	service := printService(incident, renderer, artifacts, printTestSource())
	if _, err := service.GenerateIncidentBrief(context.Background(), printPrincipal(true), "INC-PRINT", "GENERAL", nil, domain.PageSizeA4, domain.PrintMonochrome, time.Now()); err == nil {
		t.Fatal("renderer failure accepted")
	}
	if artifacts.writes != 0 {
		t.Fatalf("renderer failure replaced last-good output: writes=%d", artifacts.writes)
	}
}

func printTestSource() domain.PrintSourceMark {
	return domain.PrintSourceMark{LedgerSequence: 11, LedgerHash: "0123456789abcdef", ConfigurationVersion: "cfg-11", GeneratedAt: time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC), TimeStatus: "trusted"}
}
