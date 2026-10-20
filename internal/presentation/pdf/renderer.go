package pdf

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/goregular"
)

const (
	margin      = 42.0
	footerSpace = 34.0
	bodySize    = 9.5
	lineHeight  = 12.0
)

type Renderer struct{}

type document struct {
	pdf      gopdf.GoPdf
	pageW    float64
	pageH    float64
	y        float64
	page     int
	mode     string
	source   domain.PrintSourceMark
	exercise bool
}

func (Renderer) RenderIncidentBrief(value domain.IncidentBrief, pageSize, printMode string) ([]byte, int, error) {
	if err := domain.ValidatePrintSourceMark(value.Source); err != nil {
		return nil, 0, err
	}
	d, err := newDocument(pageSize, printMode, value.Source, value.Incident.Mode == "exercise")
	if err != nil {
		return nil, 0, err
	}
	d.addPage("INCIDENT BRIEF", value.Incident.Title)
	d.meta("Incident", value.Incident.IncidentID+" / "+value.Incident.Mode)
	d.meta("Severity", value.Incident.Severity)
	d.meta("Scope", value.Compartment)
	d.section("Situation and impact")
	d.paragraph(value.Incident.Summary)
	if len(value.OperationalRoles) > 0 {
		d.section("Operational roles")
		for _, role := range value.OperationalRoles {
			d.bullet(role.Assignment.RoleName + " - " + role.Assignment.HumanID + deputy(role.Assignment.Deputy))
		}
	}
	if len(value.Decisions) > 0 {
		d.section("Key decisions")
		for _, item := range value.Decisions {
			d.bullet(item.Decision.Decision)
		}
	}
	if len(value.Actions) > 0 {
		d.section("Priority actions")
		actions := append([]domain.ActionView(nil), value.Actions...)
		sort.SliceStable(actions, func(i, j int) bool { return actionRank(actions[i]) < actionRank(actions[j]) })
		for _, item := range actions {
			if item.Action.Status == "complete" || item.Action.Status == "skipped" {
				continue
			}
			text := fmt.Sprintf("%s - owner %s - %s / %s", item.Action.Title, item.Action.OwnerHumanID, item.Action.Priority, item.Action.Status)
			if item.Action.DueAt != nil {
				text += " - due " + item.Action.DueAt.UTC().Format(time.RFC3339)
			}
			d.bullet(text)
		}
	}
	if len(value.Obligations) > 0 {
		d.section("Active obligations")
		for _, item := range value.Obligations {
			if item.Status != domain.ObligationStatusActive {
				continue
			}
			d.bullet(fmt.Sprintf("%s - due %s - owner %s", item.Name, item.Deadline.UTC().Format(time.RFC3339), item.Owner))
		}
	}
	if value.NextCommunicationDeadline != nil {
		d.meta("Next communications deadline", value.NextCommunicationDeadline.UTC().Format(time.RFC3339))
	}
	if len(value.Unknowns) > 0 || len(value.Hypotheses) > 0 {
		d.section("Unknowns / hypotheses")
		for _, item := range value.Unknowns {
			d.bullet("UNKNOWN: " + item.Uncertainty.Statement)
		}
		for _, item := range value.Hypotheses {
			d.bullet("HYPOTHESIS: " + item.Uncertainty.Statement)
		}
	}
	if len(value.Contacts) > 0 {
		d.section("Key contacts")
		for _, c := range value.Contacts {
			text := strings.TrimSpace(c.Role + " - " + c.Name + " - " + c.Primary)
			if c.Alternate != "" {
				text += " / alt " + c.Alternate
			}
			d.bullet(text)
		}
	}
	return d.finish()
}

func (Renderer) RenderEmergencyResponsePack(value domain.EmergencyResponsePack, pageSize, printMode string) ([]byte, int, error) {
	if err := domain.ValidateEmergencyResponsePack(value); err != nil {
		return nil, 0, err
	}
	d, err := newDocument(pageSize, printMode, value.Source, value.Mode == "exercise")
	if err != nil {
		return nil, 0, err
	}
	d.addPage("EMERGENCY RESPONSE PACK", value.Organisation)
	d.paragraph("Pre-generated contingency material. Use when normal CRAB access or corporate systems are unavailable. Reconcile paper records into CRAB after restoration.")
	for _, section := range value.Sections {
		d.section(section.Title)
		d.paragraph(section.Body)
	}
	for _, form := range value.Forms {
		d.addPage("BLANK CONTINGENCY FORM", form.Title)
		d.paragraph("Write clearly. Preserve original paper time separately from later CRAB transcription time.")
		for _, field := range form.Fields {
			d.formField(field)
		}
	}
	return d.finish()
}

func (Renderer) RenderEvidenceRegister(value domain.EvidenceRegisterDocument, pageSize, printMode string) ([]byte, int, error) {
	if err := domain.ValidatePrintSourceMark(value.Source); err != nil {
		return nil, 0, err
	}
	d, err := newDocument(pageSize, printMode, value.Source, value.Incident.Mode == "exercise")
	if err != nil {
		return nil, 0, err
	}
	d.addPage("EVIDENCE REGISTER", value.Incident.Title)
	d.meta("Incident", value.Incident.IncidentID)
	d.meta("Scope", value.Compartment)
	if len(value.Evidence) == 0 {
		d.paragraph("No evidence entries in the authorised scope.")
	}
	for _, item := range value.Evidence {
		d.section(item.EvidenceID + " - " + item.Evidence.OriginalFilename)
		d.meta("Recorded", item.RecordedAt.UTC().Format(time.RFC3339)+" / "+item.TimeStatus)
		d.meta("Hash", item.Evidence.SHA256)
		d.meta("Collector / source", item.Evidence.Collector+" / "+item.Evidence.Source)
		d.meta("Current handler", item.Evidence.CurrentHandler)
		if item.Evidence.Description != "" {
			d.paragraph(item.Evidence.Description)
		}
	}
	return d.finish()
}

func (Renderer) RenderTimeline(value domain.TimelineDocument, pageSize, printMode string) ([]byte, int, error) {
	if err := domain.ValidatePrintSourceMark(value.Source); err != nil {
		return nil, 0, err
	}
	d, err := newDocument(pageSize, printMode, value.Source, value.Incident.Mode == "exercise")
	if err != nil {
		return nil, 0, err
	}
	d.addPage("INCIDENT TIMELINE", value.Incident.Title)
	d.meta("Incident", value.Incident.IncidentID)
	d.meta("Scope", value.Compartment)
	for _, item := range value.Entries {
		when := item.RecordedAt.UTC().Format(time.RFC3339)
		if item.OccurredAt != nil {
			when = item.OccurredAt.UTC().Format(time.RFC3339) + " occurred / " + when + " recorded"
		}
		d.section(fmt.Sprintf("#%d  %s", item.Sequence, when))
		d.paragraph(item.Summary)
		d.meta("Type / governance / time trust", item.RecordType+" / "+item.Governance+" / "+item.TimeStatus)
	}
	return d.finish()
}

func (Renderer) RenderReviewClosure(value domain.ReviewClosureDocument, pageSize, printMode string) ([]byte, int, error) {
	if err := domain.ValidatePrintSourceMark(value.Source); err != nil {
		return nil, 0, err
	}
	if strings.TrimSpace(value.Title) == "" {
		return nil, 0, errors.New("review/closure title is required")
	}
	d, err := newDocument(pageSize, printMode, value.Source, value.Incident.Mode == "exercise")
	if err != nil {
		return nil, 0, err
	}
	d.addPage(strings.ToUpper(value.Title), value.Incident.Title)
	d.meta("Incident", value.Incident.IncidentID)
	d.meta("Status", value.Status)
	for _, section := range value.Sections {
		d.section(section.Title)
		d.paragraph(section.Body)
	}
	return d.finish()
}

func newDocument(pageSize, printMode string, source domain.PrintSourceMark, exercise bool) (*document, error) {
	if err := domain.ValidatePageSize(pageSize); err != nil {
		return nil, err
	}
	if err := domain.ValidatePrintMode(printMode); err != nil {
		return nil, err
	}
	w, h := 595.0, 842.0
	if pageSize == domain.PageSizeLetter {
		w, h = 612, 792
	}
	d := &document{pageW: w, pageH: h, mode: printMode, source: source, exercise: exercise}
	d.pdf.Start(gopdf.Config{Unit: gopdf.UnitPT, PageSize: gopdf.Rect{W: w, H: h}})
	if err := d.pdf.AddTTFFontData("go", goregular.TTF); err != nil {
		return nil, err
	}
	if err := d.pdf.SetFont("go", "", bodySize); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *document) addPage(title, subtitle string) {
	d.pdf.AddPage()
	d.page++
	d.y = margin
	if d.exercise {
		d.text("EXERCISE - NOT A LIVE INCIDENT", 11, true)
		d.y += 4
	}
	d.text(title, 18, true)
	if subtitle != "" {
		d.text(subtitle, 11, false)
	}
	d.text(strings.Repeat("-", 72), 7.5, false)
	d.y += 4
}

func (d *document) section(title string) {
	d.ensure(34)
	d.y += 5
	d.text(strings.ToUpper(title), 11, true)
	d.y += 2
}

func (d *document) paragraph(value string) {
	for _, line := range d.wrap(value, d.pageW-2*margin) {
		d.ensure(lineHeight)
		d.text(line, bodySize, false)
	}
	d.y += 3
}

func (d *document) bullet(value string) {
	lines := d.wrap(value, d.pageW-2*margin-12)
	for i, line := range lines {
		d.ensure(lineHeight)
		prefix := "  "
		if i == 0 {
			prefix = "- "
		}
		d.text(prefix+line, bodySize, false)
	}
	d.y += 1
}

func (d *document) meta(label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	d.paragraph(label + ": " + value)
}

func (d *document) formField(label string) {
	d.ensure(54)
	d.text(label, 9, true)
	d.text(strings.Repeat("_", 76), 8, false)
	d.text(strings.Repeat("_", 76), 8, false)
	d.y += 6
}

func (d *document) ensure(height float64) {
	if d.y+height <= d.pageH-margin-footerSpace {
		return
	}
	d.footer()
	d.pdf.AddPage()
	d.page++
	d.y = margin
	if d.exercise {
		d.text("EXERCISE - NOT A LIVE INCIDENT", 11, true)
		d.y += 4
	}
	d.text("CONTINUED", 9, true)
	d.y += 4
}

func (d *document) text(value string, size float64, accent bool) {
	_ = d.pdf.SetFont("go", "", size)
	if d.mode == domain.PrintColour && accent {
		d.pdf.SetTextColor(28, 92, 64)
	} else {
		d.pdf.SetTextColor(24, 24, 24)
	}
	d.pdf.SetXY(margin, d.y)
	_ = d.pdf.Text(value)
	d.y += size + 3
}

func (d *document) wrap(value string, width float64) []string {
	value = strings.ReplaceAll(value, "\r", "")
	paragraphs := strings.Split(value, "\n")
	var out []string
	for _, p := range paragraphs {
		words := strings.Fields(p)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := words[0]
		for _, word := range words[1:] {
			candidate := line + " " + word
			w, err := d.pdf.MeasureTextWidth(candidate)
			if err == nil && w <= width {
				line = candidate
				continue
			}
			out = append(out, line)
			line = word
		}
		out = append(out, line)
	}
	return out
}

func (d *document) footer() {
	oldY := d.y
	d.y = d.pageH - 25
	shortHash := d.source.LedgerHash
	if len(shortHash) > 12 {
		shortHash = shortHash[:12]
	}
	status := strings.ToUpper(d.source.TimeStatus)
	footer := fmt.Sprintf("Page %d | ledger %d/%s | config %s | generated %s | time %s", d.page, d.source.LedgerSequence, shortHash, d.source.ConfigurationVersion, d.source.GeneratedAt.UTC().Format(time.RFC3339), status)
	d.text(footer, 6.7, false)
	d.y = oldY
}

func (d *document) finish() ([]byte, int, error) {
	if d.page == 0 {
		return nil, 0, errors.New("no PDF pages generated")
	}
	d.footer()
	bytes, err := d.pdf.GetBytesPdfReturnErr()
	if err != nil {
		return nil, 0, err
	}
	return bytes, d.page, nil
}

func actionRank(value domain.ActionView) string {
	priority := map[string]string{"critical": "0", "high": "1", "medium": "2", "low": "3"}[strings.ToLower(value.Action.Priority)]
	if priority == "" {
		priority = "4"
	}
	due := "9999"
	if value.Action.DueAt != nil {
		due = value.Action.DueAt.UTC().Format(time.RFC3339)
	}
	return priority + "|" + due + "|" + value.RecordID
}

func deputy(value bool) string {
	if value {
		return " (deputy)"
	}
	return ""
}
