package pdf

import (
	_ "embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// Storm Blue print design (UAT2-BUILD-NOTES 3.5): clear CRAB identifier,
// strong title, compact metadata, readable tables, restrained colour and
// meaning that survives black-and-white printing.

//go:embed assets/mark-colour.png
var markColour []byte

//go:embed assets/mark-mono.png
var markMono []byte

const (
	margin      = 44.0
	footerSpace = 40.0
	bodySize    = 9.5
	lineHeight  = 12.5
)

type rgb struct{ r, g, b uint8 }

var (
	stormBlue = rgb{51, 74, 94}
	deepInk   = rgb{23, 33, 43}
	slate     = rgb{104, 119, 134}
	mist      = rgb{214, 219, 223} // Mist, deepened slightly so rules survive printing
	paperFill = rgb{247, 247, 245}
	black     = rgb{0, 0, 0}
	grey      = rgb{80, 80, 80}
	lightRule = rgb{170, 170, 170}
	lightFill = rgb{242, 242, 242}
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
	kind     string // document type for the running header/footer
	ref      string // incident or organisation reference
	mark     gopdf.ImageHolder
}

func (Renderer) RenderIncidentBrief(value domain.IncidentBrief, pageSize, printMode string) ([]byte, int, error) {
	if err := domain.ValidatePrintSourceMark(value.Source); err != nil {
		return nil, 0, err
	}
	d, err := newDocument(pageSize, printMode, value.Source, value.Incident.Mode == "exercise")
	if err != nil {
		return nil, 0, err
	}
	d.kind, d.ref = "Incident Brief", value.Incident.IncidentID
	d.addPage("Incident Brief", value.Incident.Title)
	d.metaBar([][2]string{
		{"Incident", value.Incident.IncidentID},
		{"Mode", strings.ToUpper(modeWord(value.Incident.Mode))},
		{"Severity", dash(value.Incident.Severity)},
		{"Scope", value.Compartment},
		{"Generated", value.Source.GeneratedAt.UTC().Format("2 Jan 2006 15:04 UTC")},
		{"Time", timeTrustLabel(value.Source.TimeStatus)},
	})
	d.section("Situation and impact")
	d.paragraph(value.Incident.Summary)
	if len(value.Facts) > 0 {
		d.section("What we know")
		for _, item := range value.Facts {
			text := item.Fact.Statement
			if item.Fact.Source != "" {
				text += " (source: " + item.Fact.Source + ")"
			}
			d.bullet("FACT", text)
		}
	}
	if len(value.Unknowns) > 0 || len(value.Hypotheses) > 0 {
		d.section("Unknowns and hypotheses")
		for _, item := range value.Unknowns {
			d.bullet("UNKNOWN", item.Uncertainty.Statement)
		}
		for _, item := range value.Hypotheses {
			d.bullet("HYPOTHESIS", item.Uncertainty.Statement)
		}
	}
	if len(value.OperationalRoles) > 0 {
		d.section("Who is in charge")
		rows := make([][]string, 0, len(value.OperationalRoles))
		for _, role := range value.OperationalRoles {
			rows = append(rows, []string{role.Assignment.RoleName, role.Assignment.HumanID + deputy(role.Assignment.Deputy)})
		}
		d.table([]string{"Role", "Person"}, []float64{0.45, 0.55}, rows)
	}
	if len(value.Decisions) > 0 {
		d.section("Key decisions")
		for _, item := range value.Decisions {
			if item.Maker != "" && item.Maker == item.Confirmer {
				d.bullet("SELF-APPROVED", item.Decision.Decision+" ("+item.Maker+", single-person window)")
				continue
			}
			d.bullet("", item.Decision.Decision)
		}
	}
	actions := append([]domain.ActionView(nil), value.Actions...)
	sort.SliceStable(actions, func(i, j int) bool { return actionRank(actions[i]) < actionRank(actions[j]) })
	var actionRows [][]string
	for _, item := range actions {
		if item.Action.Status == "complete" || item.Action.Status == "skipped" {
			continue
		}
		due := "—"
		if item.Action.DueAt != nil {
			due = item.Action.DueAt.UTC().Format("2 Jan 15:04 UTC")
		}
		title := item.Action.Title
		if item.Maker != "" && item.Maker == item.Confirmer {
			title += " [SELF-APPROVED]"
		}
		actionRows = append(actionRows, []string{title, item.Action.OwnerHumanID, item.Action.Priority, strings.ReplaceAll(item.Action.Status, "_", " "), due})
	}
	if len(actionRows) > 0 {
		d.section("Priority actions")
		d.table([]string{"Action", "Owner", "Priority", "Status", "Due"}, []float64{0.42, 0.16, 0.12, 0.12, 0.18}, actionRows)
	}
	var clockRows [][]string
	for _, item := range value.Obligations {
		if item.Status != domain.ObligationStatusActive {
			continue
		}
		due := item.Deadline.UTC().Format("2 Jan 2006 15:04 UTC")
		if item.TimeStatus != ledger.TimeTrusted {
			due += " TIME UNVERIFIED"
		}
		clockRows = append(clockRows, []string{item.Name, due, strings.ToUpper(item.Lifecycle), item.Owner})
	}
	if len(clockRows) > 0 || value.NextCommunicationDeadline != nil {
		d.section("Active clocks")
		if len(clockRows) > 0 {
			d.table([]string{"Obligation", "Deadline", "State", "Owner"}, []float64{0.34, 0.32, 0.14, 0.20}, clockRows)
		}
		if value.NextCommunicationDeadline != nil {
			d.keyValue("Next situation report due", value.NextCommunicationDeadline.UTC().Format("2 Jan 2006 15:04 UTC"))
		}
	}
	if len(value.Contacts) > 0 {
		d.section("Key contacts")
		rows := make([][]string, 0, len(value.Contacts))
		for _, c := range value.Contacts {
			rows = append(rows, []string{c.Role, c.Name, c.Primary, dash(c.Alternate)})
		}
		d.table([]string{"Role", "Name", "Primary", "Alternative"}, []float64{0.24, 0.24, 0.26, 0.26}, rows)
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
	d.kind, d.ref = "Emergency Response Pack", value.Organisation
	d.addPage("Emergency Response Pack", value.Organisation)
	d.metaBar([][2]string{
		{"Organisation", value.Organisation},
		{"Generated", value.Source.GeneratedAt.UTC().Format("2 Jan 2006 15:04 UTC")},
		{"Time", timeTrustLabel(value.Source.TimeStatus)},
	})
	d.note("Use this pack when CRAB or normal corporate systems are unavailable. Record on the blank forms at the back. Reconcile paper records into CRAB after restoration; original times are preserved.")
	for _, section := range value.Sections {
		d.section(section.Title)
		if len(section.Rows) > 0 && len(section.Columns) > 0 {
			d.table(section.Columns, section.Widths, section.Rows)
		}
		if strings.TrimSpace(section.Body) != "" {
			d.paragraph(section.Body)
		}
	}
	for _, form := range value.Forms {
		d.addPage("Blank form", form.Title)
		d.note("Write clearly. Record the time on every entry. Preserve the original paper time separately from any later CRAB transcription time.")
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
	d.kind, d.ref = "Evidence Register", value.Incident.IncidentID
	d.addPage("Evidence Register", value.Incident.Title)
	d.metaBar([][2]string{{"Incident", value.Incident.IncidentID}, {"Scope", value.Compartment}, {"Items", fmt.Sprint(len(value.Evidence))}})
	if len(value.Evidence) == 0 {
		d.paragraph("No evidence entries in the authorised scope.")
	}
	for _, item := range value.Evidence {
		d.section(item.EvidenceID + " · " + item.Evidence.OriginalFilename)
		recorded := item.RecordedAt.UTC().Format("2 Jan 2006 15:04 UTC")
		if item.TimeStatus != ledger.TimeTrusted {
			recorded += " · TIME UNVERIFIED"
		}
		d.keyValue("Recorded", recorded)
		d.keyValue("SHA-256", item.Evidence.SHA256)
		d.keyValue("Collector / source", item.Evidence.Collector+" / "+item.Evidence.Source)
		d.keyValue("Current handler", item.Evidence.CurrentHandler)
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
	d.kind, d.ref = "Incident Timeline", value.Incident.IncidentID
	d.addPage("Incident Timeline", value.Incident.Title)
	d.metaBar([][2]string{{"Incident", value.Incident.IncidentID}, {"Scope", value.Compartment}, {"Entries", fmt.Sprint(len(value.Entries))}})
	rows := make([][]string, 0, len(value.Entries))
	for _, item := range value.Entries {
		recorded := item.RecordedAt.UTC().Format("2 Jan 15:04")
		if item.TimeStatus != ledger.TimeTrusted {
			recorded += " UNVERIFIED"
		}
		occurred := "—"
		if item.OccurredAt != nil {
			occurred = item.OccurredAt.UTC().Format("2 Jan 15:04")
		}
		rows = append(rows, []string{fmt.Sprint(item.Sequence), occurred, recorded, item.Summary, item.RecordType + " · " + item.Governance})
	}
	if len(rows) > 0 {
		d.table([]string{"#", "Occurred (UTC)", "Recorded (UTC)", "Entry", "Type"}, []float64{0.07, 0.15, 0.17, 0.41, 0.20}, rows)
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
	d.kind, d.ref = value.Title, value.Incident.IncidentID
	d.addPage(value.Title, value.Incident.Title)
	d.metaBar([][2]string{{"Incident", value.Incident.IncidentID}, {"Status", value.Status}})
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
	if err := d.pdf.AddTTFFontData("gobold", gobold.TTF); err != nil {
		return nil, err
	}
	if err := d.pdf.SetFont("go", "", bodySize); err != nil {
		return nil, err
	}
	mark := markColour
	if printMode == domain.PrintMonochrome {
		mark = markMono
	}
	holder, err := gopdf.ImageHolderByBytes(mark)
	if err != nil {
		return nil, err
	}
	d.mark = holder
	return d, nil
}

// ---- colour ---------------------------------------------------------------

func (d *document) colour(c rgb) rgb {
	if d.mode != domain.PrintMonochrome {
		return c
	}
	switch c {
	case stormBlue, deepInk:
		return black
	case slate:
		return grey
	case mist:
		return lightRule
	case paperFill:
		return lightFill
	}
	return c
}

func (d *document) textColour(c rgb) {
	c = d.colour(c)
	d.pdf.SetTextColor(c.r, c.g, c.b)
}

func (d *document) rule(x1, x2, y float64, c rgb, width float64) {
	c = d.colour(c)
	d.pdf.SetStrokeColor(c.r, c.g, c.b)
	d.pdf.SetLineWidth(width)
	d.pdf.Line(x1, y, x2, y)
}

func (d *document) fill(x, y, w, h float64, c rgb) {
	c = d.colour(c)
	d.pdf.SetFillColor(c.r, c.g, c.b)
	d.pdf.RectFromUpperLeftWithStyle(x, y, w, h, "F")
}

// ---- page furniture -------------------------------------------------------

func (d *document) addPage(title, subtitle string) {
	if d.page > 0 {
		d.footer()
	}
	d.pdf.AddPage()
	d.page++
	d.y = margin
	d.masthead()
	d.exerciseBanner()
	d.write(title, 20, true, deepInk, margin)
	d.y += 2
	if subtitle != "" {
		for _, line := range d.wrapAt(subtitle, d.pageW-2*margin, 11.5, false) {
			d.write(line, 11.5, false, slate, margin)
		}
	}
	d.y += 6
	d.rule(margin, d.pageW-margin, d.y, stormBlue, 1.2)
	d.y += 12
}

// masthead is the CRAB identifier on every page: mark, wordmark and the
// document type, so a loose sheet is still recognisable.
func (d *document) masthead() {
	_ = d.pdf.ImageByHolder(d.mark, margin, d.y-2, &gopdf.Rect{W: 20, H: 20})
	d.at(margin+25, d.y+3, "CRAB", 12, true, deepInk)
	label := strings.ToUpper(d.kind)
	if d.ref != "" {
		label += "  ·  " + d.ref
	}
	_ = d.pdf.SetFont("gobold", "", 7.5)
	width, _ := d.pdf.MeasureTextWidth(label)
	d.at(d.pageW-margin-width, d.y+5, label, 7.5, true, stormBlue)
	d.y += 22
	d.rule(margin, d.pageW-margin, d.y, mist, 0.6)
	d.y += 12
}

func (d *document) exerciseBanner() {
	if !d.exercise {
		return
	}
	h := 20.0
	d.fill(margin, d.y, d.pageW-2*margin, h, paperFill)
	c := d.colour(deepInk)
	d.pdf.SetStrokeColor(c.r, c.g, c.b)
	d.pdf.SetLineWidth(1)
	d.pdf.RectFromUpperLeftWithStyle(margin, d.y, d.pageW-2*margin, h, "D")
	d.at(margin+8, d.y+6, "EXERCISE — NOT A LIVE INCIDENT. No real obligations start and no external notifications are sent.", 8.5, true, deepInk)
	d.y += h + 10
}

func (d *document) continuation() {
	d.footer()
	d.pdf.AddPage()
	d.page++
	d.y = margin
	d.masthead()
	d.exerciseBanner()
	d.write("Continued", 8, true, slate, margin)
	d.y += 4
}

// timeTrustLabel states time trust in words that do not depend on colour.
func timeTrustLabel(status string) string {
	switch status {
	case ledger.TimeTrusted:
		return "TIME VERIFIED"
	case ledger.TimeDrifting:
		return "TIME DRIFTING"
	}
	return "TIME UNVERIFIED"
}

func (d *document) footer() {
	oldY := d.y
	y := d.pageH - 30
	d.rule(margin, d.pageW-margin, y, mist, 0.6)
	shortHash := d.source.LedgerHash
	if len(shortHash) > 12 {
		shortHash = shortHash[:12]
	}
	left := fmt.Sprintf("CRAB · %s · ledger %d/%s · config %s · generated %s · %s", d.kind, d.source.LedgerSequence, shortHash, d.source.ConfigurationVersion, d.source.GeneratedAt.UTC().Format(time.RFC3339), timeTrustLabel(d.source.TimeStatus))
	if d.exercise {
		left = "EXERCISE · " + left
	}
	d.at(margin, y+7, left, 6.6, false, slate)
	page := fmt.Sprintf("Page %d", d.page)
	_ = d.pdf.SetFont("gobold", "", 7)
	width, _ := d.pdf.MeasureTextWidth(page)
	d.at(d.pageW-margin-width, y+7, page, 7, true, deepInk)
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

// ---- content blocks ------------------------------------------------------

func (d *document) section(title string) {
	d.ensure(44)
	d.y += 8
	d.write(title, 12, true, stormBlue, margin)
	d.y += 1
	d.rule(margin, d.pageW-margin, d.y, mist, 0.6)
	d.y += 7
}

func (d *document) paragraph(value string) {
	for _, line := range d.wrap(value, d.pageW-2*margin) {
		d.ensure(lineHeight)
		d.write(line, bodySize, false, deepInk, margin)
	}
	d.y += 4
}

// note is secondary guidance text.
func (d *document) note(value string) {
	for _, line := range d.wrapAt(value, d.pageW-2*margin, 8.8, false) {
		d.ensure(lineHeight)
		d.write(line, 8.8, false, slate, margin)
	}
	d.y += 4
}

// bullet renders an item with an optional text label (FACT, UNKNOWN...) so
// meaning never depends on colour.
func (d *document) bullet(label, value string) {
	indent := 12.0
	if label != "" {
		_ = d.pdf.SetFont("gobold", "", 7.5)
		w, _ := d.pdf.MeasureTextWidth(label)
		indent = w + 10
	}
	lines := d.wrap(value, d.pageW-2*margin-indent)
	for i, line := range lines {
		d.ensure(lineHeight)
		if i == 0 {
			if label != "" {
				d.at(margin, d.y+1, label, 7.5, true, stormBlue)
			} else {
				d.at(margin+2, d.y, "•", bodySize, false, stormBlue)
			}
		}
		d.write(line, bodySize, false, deepInk, margin+indent)
	}
	d.y += 2
}

func (d *document) keyValue(label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	labelW := 128.0
	lines := d.wrap(value, d.pageW-2*margin-labelW)
	for i, line := range lines {
		d.ensure(lineHeight)
		if i == 0 {
			d.at(margin, d.y, label, 8.5, true, slate)
		}
		d.write(line, bodySize, false, deepInk, margin+labelW)
	}
	d.y += 2
}

// meta is kept for compatibility with simple label/value lines.
func (d *document) meta(label, value string) { d.keyValue(label, value) }

// metaBar is the compact metadata strip under the title: labels above values
// in a lightly shaded box.
func (d *document) metaBar(items [][2]string) {
	var kept [][2]string
	for _, item := range items {
		if strings.TrimSpace(item[1]) != "" {
			kept = append(kept, item)
		}
	}
	if len(kept) == 0 {
		return
	}
	perRow := 3
	if len(kept) < perRow {
		perRow = len(kept)
	}
	rows := (len(kept) + perRow - 1) / perRow
	cellW := (d.pageW - 2*margin) / float64(perRow)
	h := float64(rows)*26 + 8
	d.ensure(h + 8)
	d.fill(margin, d.y, d.pageW-2*margin, h, paperFill)
	top := d.y + 6
	for i, item := range kept {
		x := margin + 8 + float64(i%perRow)*cellW
		y := top + float64(i/perRow)*26
		d.at(x, y, strings.ToUpper(item[0]), 6.8, true, slate)
		value := item[1]
		for _, line := range d.wrapAt(value, cellW-14, 9.5, true) {
			value = line
			break
		}
		d.at(x, y+10, value, 9.5, true, deepInk)
	}
	d.y += h + 10
}

// table draws a readable table with a shaded header and thin row rules.
func (d *document) table(headers []string, widths []float64, rows [][]string) {
	total := d.pageW - 2*margin
	cols := len(headers)
	if len(widths) != cols {
		widths = make([]float64, cols)
		for i := range widths {
			widths[i] = 1 / float64(cols)
		}
	}
	colW := make([]float64, cols)
	for i, w := range widths {
		colW[i] = w * total
	}
	pad := 4.0
	drawHeader := func() {
		h := 16.0
		d.fill(margin, d.y, total, h, paperFill)
		x := margin
		for i, head := range headers {
			d.at(x+pad, d.y+5, strings.ToUpper(head), 6.8, true, slate)
			x += colW[i]
		}
		d.y += h
		d.rule(margin, margin+total, d.y, mist, 0.6)
	}
	d.ensure(16 + lineHeight + 6)
	drawHeader()
	size := 8.8
	for _, row := range rows {
		cells := make([][]string, cols)
		lines := 1
		for i := 0; i < cols; i++ {
			text := ""
			if i < len(row) {
				text = row[i]
			}
			cells[i] = d.wrapAt(text, colW[i]-2*pad, size, i == 0)
			if len(cells[i]) > lines {
				lines = len(cells[i])
			}
		}
		h := float64(lines)*(size+3) + 7
		if d.y+h > d.pageH-margin-footerSpace {
			d.continuation()
			drawHeader()
		}
		x := margin
		for i := 0; i < cols; i++ {
			y := d.y + 4
			for _, line := range cells[i] {
				d.at(x+pad, y, line, size, i == 0, deepInk)
				y += size + 3
			}
			x += colW[i]
		}
		d.y += h
		d.rule(margin, margin+total, d.y, mist, 0.5)
	}
	d.y += 8
}

// formField draws a label and two writable ruled lines.
func (d *document) formField(label string) {
	d.ensure(58)
	d.write(label, 9, true, deepInk, margin)
	d.y += 14
	d.rule(margin, d.pageW-margin, d.y, slate, 0.5)
	d.y += 20
	d.rule(margin, d.pageW-margin, d.y, slate, 0.5)
	d.y += 14
}

func (d *document) ensure(height float64) {
	if d.y+height <= d.pageH-margin-footerSpace {
		return
	}
	d.continuation()
}

// ---- text primitives -----------------------------------------------------

func (d *document) font(size float64, bold bool) {
	if bold {
		_ = d.pdf.SetFont("gobold", "", size)
		return
	}
	_ = d.pdf.SetFont("go", "", size)
}

// at draws one line whose top is y, without moving the cursor. gopdf's Text
// places the baseline at the current y, so the ascent is added here.
func (d *document) at(x, y float64, value string, size float64, bold bool, c rgb) {
	d.font(size, bold)
	d.textColour(c)
	d.pdf.SetXY(x, y+size*0.8)
	_ = d.pdf.Text(value)
}

// write draws one line at the cursor and advances it.
func (d *document) write(value string, size float64, bold bool, c rgb, x float64) {
	d.at(x, d.y, value, size, bold, c)
	d.y += size + 3
}

// text is retained for callers that draw a plain line.
func (d *document) text(value string, size float64, accent bool) {
	c := deepInk
	if accent {
		c = stormBlue
	}
	d.write(value, size, accent, c, margin)
}

func (d *document) wrap(value string, width float64) []string {
	return d.wrapAt(value, width, bodySize, false)
}

func (d *document) wrapAt(value string, width, size float64, bold bool) []string {
	d.font(size, bold)
	value = strings.ReplaceAll(value, "\r", "")
	paragraphs := strings.Split(value, "\n")
	var out []string
	for _, p := range paragraphs {
		words := strings.Fields(p)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		var bounded []string
		for _, word := range words {
			bounded = append(bounded, d.splitLongWord(word, width)...)
		}
		line := bounded[0]
		for _, word := range bounded[1:] {
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

func (d *document) splitLongWord(word string, width float64) []string {
	measured, err := d.pdf.MeasureTextWidth(word)
	if err == nil && measured <= width {
		return []string{word}
	}
	runes := []rune(word)
	if len(runes) == 0 {
		return []string{""}
	}
	var parts []string
	start := 0
	for start < len(runes) {
		end := start + 1
		for end <= len(runes) {
			candidate := string(runes[start:end])
			measured, measureErr := d.pdf.MeasureTextWidth(candidate)
			if measureErr != nil || measured > width {
				break
			}
			end++
		}
		if end == start+1 {
			parts = append(parts, string(runes[start:end]))
			start = end
			continue
		}
		parts = append(parts, string(runes[start:end-1]))
		start = end - 1
	}
	return parts
}

func actionRank(value domain.ActionView) string {
	priority := map[string]string{"critical": "0", "high": "1", "medium": "2", "low": "3", "p1": "0", "p2": "1", "p3": "2"}[strings.ToLower(value.Action.Priority)]
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

func modeWord(mode string) string {
	if mode == "exercise" {
		return "Exercise"
	}
	return "Live"
}

func dash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}
