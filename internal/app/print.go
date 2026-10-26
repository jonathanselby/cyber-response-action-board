package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type PrintIncidentReader interface {
	CommonOperatingPicture(context.Context, authz.Principal, string, string) (domain.OperatingPicture, bool, error)
}

type PrintEvidenceReader interface {
	Register(context.Context, authz.Principal, string, string) ([]domain.EvidenceView, error)
}

type PrintSourceReader interface {
	LedgerSource(context.Context, time.Time, string) (domain.PrintSourceMark, error)
	NextCommunicationDeadline(context.Context, string, string) (*time.Time, error)
}

type PDFRenderer interface {
	RenderIncidentBrief(domain.IncidentBrief, string, string) ([]byte, int, error)
	RenderEmergencyResponsePack(domain.EmergencyResponsePack, string, string) ([]byte, int, error)
	RenderEvidenceRegister(domain.EvidenceRegisterDocument, string, string) ([]byte, int, error)
	RenderTimeline(domain.TimelineDocument, string, string) ([]byte, int, error)
	RenderReviewClosure(domain.ReviewClosureDocument, string, string) ([]byte, int, error)
}

type ArtifactWriteResult struct {
	PrimaryPath      string
	SecondaryPath    string
	SecondaryWarning string
}

type ArtifactStore interface {
	WriteArtifact(string, []byte) (ArtifactWriteResult, error)
}

type PrintResult struct {
	Artifact ArtifactWriteResult
	Pages    int
	Source   domain.PrintSourceMark
}

type PrintService struct {
	Incident             PrintIncidentReader
	Evidence             PrintEvidenceReader
	Source               PrintSourceReader
	Renderer             PDFRenderer
	Artifacts            ArtifactStore
	ConfigurationVersion string
	// Organisation supplies confirmed emergency configuration after print
	// authority has been checked (UAT 2 item 2).
	Organisation interface {
		EmergencyForPrint(context.Context) (domain.EmergencyConfiguration, bool, error)
	}
	// TimeStatus reports the one clock at generation time. When set, the
	// printed time-trust mark is the weaker of the ledger tail and the clock.
	TimeStatus func() string
}

func (s PrintService) ledgerSource(ctx context.Context, now time.Time) (domain.PrintSourceMark, error) {
	source, err := s.Source.LedgerSource(ctx, now, s.ConfigurationVersion)
	if err != nil {
		return domain.PrintSourceMark{}, err
	}
	if s.TimeStatus != nil {
		source.TimeStatus = ledger.WeakerTimeStatus(source.TimeStatus, s.TimeStatus())
	}
	return source, nil
}

func (s PrintService) GenerateIncidentBrief(ctx context.Context, principal authz.Principal, incidentID, compartment string, contacts []domain.PrintContact, pageSize, printMode string, now time.Time) (PrintResult, error) {
	if err := s.ready(); err != nil {
		return PrintResult{}, err
	}
	if err := s.authorisePrint(principal, incidentID, compartment); err != nil {
		return PrintResult{}, err
	}
	picture, found, err := s.Incident.CommonOperatingPicture(ctx, principal, incidentID, compartment)
	if err != nil {
		return PrintResult{}, err
	}
	if !found {
		return PrintResult{}, ErrAuthorisationDenied
	}
	source, err := s.ledgerSource(ctx, now)
	if err != nil {
		return PrintResult{}, err
	}
	nextComms, err := s.Source.NextCommunicationDeadline(ctx, incidentID, compartment)
	if err != nil {
		return PrintResult{}, err
	}
	brief := domain.IncidentBrief{
		Source: source, Incident: picture.Incident, Compartment: normaliseCompartment(compartment),
		Facts: picture.Facts, Unknowns: picture.Unknowns, Hypotheses: picture.Hypotheses,
		Decisions: picture.Decisions, Actions: picture.Actions, Obligations: picture.Obligations,
		OperationalRoles: picture.OperationalRoles, NextCommunicationDeadline: nextComms,
		Contacts: append([]domain.PrintContact(nil), contacts...),
	}
	if len(brief.Contacts) == 0 && s.Organisation != nil {
		if config, found, err := s.Organisation.EmergencyForPrint(ctx); err != nil {
			return PrintResult{}, err
		} else if found {
			brief.Contacts = domain.BriefContacts(config)
		}
	}
	bytes, pages, err := s.Renderer.RenderIncidentBrief(brief, pageSize, printMode)
	if err != nil {
		return PrintResult{}, err
	}
	return s.write("incident-brief-"+incidentID+".pdf", bytes, pages, source)
}

func (s PrintService) GenerateEvidenceRegister(ctx context.Context, principal authz.Principal, incidentID, compartment, pageSize, printMode string, now time.Time) (PrintResult, error) {
	if err := s.ready(); err != nil {
		return PrintResult{}, err
	}
	if s.Evidence == nil {
		return PrintResult{}, errors.New("print evidence reader is required")
	}
	if err := s.authorisePrint(principal, incidentID, compartment); err != nil {
		return PrintResult{}, err
	}
	picture, found, err := s.Incident.CommonOperatingPicture(ctx, principal, incidentID, compartment)
	if err != nil || !found {
		if err != nil {
			return PrintResult{}, err
		}
		return PrintResult{}, ErrAuthorisationDenied
	}
	evidence, err := s.Evidence.Register(ctx, principal, incidentID, compartment)
	if err != nil {
		return PrintResult{}, err
	}
	source, err := s.ledgerSource(ctx, now)
	if err != nil {
		return PrintResult{}, err
	}
	doc := domain.EvidenceRegisterDocument{Source: source, Incident: picture.Incident, Compartment: normaliseCompartment(compartment), Evidence: evidence}
	bytes, pages, err := s.Renderer.RenderEvidenceRegister(doc, pageSize, printMode)
	if err != nil {
		return PrintResult{}, err
	}
	return s.write("evidence-register-"+incidentID+".pdf", bytes, pages, source)
}

func (s PrintService) GenerateTimeline(ctx context.Context, principal authz.Principal, incidentID, compartment, pageSize, printMode string, now time.Time) (PrintResult, error) {
	if err := s.ready(); err != nil {
		return PrintResult{}, err
	}
	if err := s.authorisePrint(principal, incidentID, compartment); err != nil {
		return PrintResult{}, err
	}
	picture, found, err := s.Incident.CommonOperatingPicture(ctx, principal, incidentID, compartment)
	if err != nil || !found {
		if err != nil {
			return PrintResult{}, err
		}
		return PrintResult{}, ErrAuthorisationDenied
	}
	source, err := s.ledgerSource(ctx, now)
	if err != nil {
		return PrintResult{}, err
	}
	doc := domain.TimelineDocument{Source: source, Incident: picture.Incident, Compartment: normaliseCompartment(compartment), Entries: picture.Timeline}
	bytes, pages, err := s.Renderer.RenderTimeline(doc, pageSize, printMode)
	if err != nil {
		return PrintResult{}, err
	}
	return s.write("timeline-"+incidentID+".pdf", bytes, pages, source)
}

func (s PrintService) GenerateEmergencyResponsePack(ctx context.Context, principal authz.Principal, pack domain.EmergencyResponsePack, pageSize, printMode string, now time.Time) (PrintResult, error) {
	if err := s.ready(); err != nil {
		return PrintResult{}, err
	}
	if !principal.Authenticated || strings.TrimSpace(principal.HumanID) == "" || principal.Role != authz.RoleMajor || !principal.Outputs[authz.ActionPrint] {
		return PrintResult{}, ErrAuthorisationDenied
	}
	source, err := s.ledgerSource(ctx, now)
	if err != nil {
		return PrintResult{}, err
	}
	pack.Source = source
	if s.Organisation != nil {
		config, found, err := s.Organisation.EmergencyForPrint(ctx)
		if err != nil {
			return PrintResult{}, err
		}
		if found {
			pack.Organisation = config.Organisation
			pack.Sections = append(domain.EmergencyPackSections(config), pack.Sections...)
		}
	}
	if err := domain.ValidateEmergencyResponsePack(pack); err != nil {
		return PrintResult{}, err
	}
	bytes, pages, err := s.Renderer.RenderEmergencyResponsePack(pack, pageSize, printMode)
	if err != nil {
		return PrintResult{}, err
	}
	return s.write("emergency-response-pack.pdf", bytes, pages, source)
}

func (s PrintService) GenerateReviewClosure(ctx context.Context, principal authz.Principal, incidentID, compartment, title, status string, sections []domain.PackSection, pageSize, printMode string, now time.Time) (PrintResult, error) {
	if err := s.ready(); err != nil {
		return PrintResult{}, err
	}
	if err := s.authorisePrint(principal, incidentID, compartment); err != nil {
		return PrintResult{}, err
	}
	picture, found, err := s.Incident.CommonOperatingPicture(ctx, principal, incidentID, compartment)
	if err != nil || !found {
		if err != nil {
			return PrintResult{}, err
		}
		return PrintResult{}, ErrAuthorisationDenied
	}
	source, err := s.ledgerSource(ctx, now)
	if err != nil {
		return PrintResult{}, err
	}
	doc := domain.ReviewClosureDocument{Source: source, Incident: picture.Incident, Title: title, Status: status, Sections: append([]domain.PackSection(nil), sections...)}
	bytes, pages, err := s.Renderer.RenderReviewClosure(doc, pageSize, printMode)
	if err != nil {
		return PrintResult{}, err
	}
	return s.write("review-closure-"+incidentID+".pdf", bytes, pages, source)
}

func (s IncidentService) RecordPaperTranscription(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, value domain.PaperTranscription, transcribedAt time.Time) (ledger.Record, error) {
	if err := s.authorise(principal, authz.ActionRecord, scope); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.RecordPaperTranscription(scope, recordID, principal.HumanID, transcribedAt, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendIncident(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s PrintService) authorisePrint(principal authz.Principal, incidentID, compartment string) error {
	compartment = normaliseCompartment(compartment)
	if decision := authz.Authorise(principal, authz.ActionPrint, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}); !decision.Allowed {
		return ErrAuthorisationDenied
	}
	return nil
}

func (s PrintService) write(name string, bytes []byte, pages int, source domain.PrintSourceMark) (PrintResult, error) {
	if len(bytes) == 0 || pages < 1 {
		return PrintResult{}, errors.New("renderer returned empty print artefact")
	}
	result, err := s.Artifacts.WriteArtifact(name, bytes)
	if err != nil {
		return PrintResult{}, err
	}
	return PrintResult{Artifact: result, Pages: pages, Source: source}, nil
}

func (s PrintService) ready() error {
	if s.Incident == nil || s.Source == nil || s.Renderer == nil || s.Artifacts == nil || strings.TrimSpace(s.ConfigurationVersion) == "" {
		return errors.New("print service requires incident/source/renderer/artifact store and configuration version")
	}
	return nil
}

func normaliseCompartment(value string) string {
	if strings.TrimSpace(value) == "" {
		return "GENERAL"
	}
	return value
}
