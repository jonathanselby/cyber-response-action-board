package web

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/go-app-shell/shell"
)

const sessionCookie = "crab_session"

// Action is a header action rendered by the shared "actions" template.
type Action struct {
	Label   string
	URL     string
	Method  string // "" for a link, "post" for a form button
	Primary bool
}

// screen describes one browser screen. Handlers supply data; page() owns the
// frame, so incident context, exercise marking and status can never be omitted.
type screen struct {
	nav      string
	title    string
	summary  string
	tmpl     string
	incident bool
	load     func(r *http.Request, token string, frame FrameView) (data any, actions []Action, err error)
}

// pageData is what every screen template receives.
type pageData struct {
	Frame       FrameView
	Data        any
	Form        url.Values
	Error       string
	Open        string
	Notice      string
	IncidentID  string
	Compartment string
}

var messages = map[string]string{
	"fact-recorded":        "Fact recorded.",
	"uncertainty-recorded": "Recorded.",
	"recorded":             "Recorded as supplied.",
	"proposed":             "Proposed. It takes effect when another eligible person confirms.",
	"decided-confirmed":    "Confirmed.",
	"decided-rejected":     "Rejected.",
	"evidence-preserved":   "Evidence preserved. SHA-256 recorded.",
	"report-recorded":      "First report recorded.",
	"handover-recorded":    "Handover recorded.",
	"notified":             "Notification attempts recorded. See the outcomes below.",
	"drafted":              "Draft recorded.",
	"triaged":              "Triage recorded.",
	"decided":              "Decision recorded.",
}

type navItem struct{ key, label, path string }

var responseNav = []navItem{{"incidents", "Command Summary", "/"}, {"approvals", "Approvals", "/approvals"}, {"intake", "First reports", "/intake"}}
var incidentNav = []navItem{{"board", "Board", ""}, {"evidence", "Evidence", "/evidence"}, {"communications", "Communications", "/communications"}, {"access", "Access", "/access"}, {"close", "Close-out", "/close"}}
var systemNav = []navItem{{"readiness", "Readiness", "/readiness"}, {"obligations", "Obligation library", "/obligations"}, {"access-admin", "Access admin", "/access"}}

func (s *Server) navigation(frame FrameView, current string) []shell.NavGroup {
	group := func(label string, items []navItem, prefix string) shell.NavGroup {
		g := shell.NavGroup{Label: label}
		for _, item := range items {
			g.Items = append(g.Items, shell.NavItem{Label: item.label, URL: prefix + item.path, Current: item.key == current})
		}
		return g
	}
	groups := []shell.NavGroup{group("Response", responseNav, "")}
	if frame.Incident != nil {
		groups = append(groups, group(frame.Incident.ID, incidentNav, "/incident/"+url.PathEscape(frame.Incident.ID)))
	}
	groups = append(groups, group("System", systemNav, ""))
	return groups
}

// statusLine reports only abnormal states, most important first (UI notes 7.1).
func statusLine(frame FrameView) string {
	var items []string
	if frame.Incident != nil && frame.Incident.Mode == "exercise" {
		items = append(items, "EXERCISE")
	}
	st := frame.Status
	if st.IntegrityFailed {
		items = append(items, "Integrity failed: read only")
	}
	if st.BreakGlass {
		items = append(items, "Break-glass active")
	}
	if st.Isolate {
		items = append(items, "Isolate on")
	}
	switch st.TimeStatus {
	case "untrusted":
		items = append(items, "Time unverified")
	case "drifting":
		items = append(items, "Time drifting")
	}
	if st.PendingApprovals == 1 {
		items = append(items, "1 approval")
	} else if st.PendingApprovals > 1 {
		items = append(items, strconv.Itoa(st.PendingApprovals)+" approvals")
	}
	if st.PackStale {
		items = append(items, "Paper pack stale")
	}
	if st.ReadinessFail {
		items = append(items, "Readiness attention")
	}
	if st.ModelUnavailable {
		items = append(items, "Assistance unavailable")
	}
	if len(items) == 0 {
		return "Ready"
	}
	if len(items) > 4 {
		return strings.Join(items[:4], " · ") + " · +" + strconv.Itoa(len(items)-4)
	}
	return strings.Join(items, " · ")
}

func headerContext(frame FrameView) string {
	if frame.Incident == nil {
		return ""
	}
	parts := []string{strings.ToUpper(modeLabel(frame.Incident.Mode)), frame.Incident.ID}
	if frame.Incident.Title != "" {
		parts = append(parts, frame.Incident.Title)
	}
	if frame.Incident.Phase != "" {
		parts = append(parts, frame.Incident.Phase)
	}
	return strings.Join(parts, " · ")
}

func modeLabel(mode string) string {
	if mode == "exercise" {
		return "Exercise"
	}
	return "Live"
}

// page renders one screen inside the shell frame. It holds the package's only
// conversion of executed template output to template.HTML.
func (s *Server) page(w http.ResponseWriter, status int, sc screen, data pageData, actions []Action) {
	out, err := s.compose(sc, data, actions)
	if err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(out)
}

// compose builds the complete framed document for a screen.
func (s *Server) compose(sc screen, data pageData, actions []Action) ([]byte, error) {
	var body, actionHTML bytes.Buffer
	if err := s.templates.ExecuteTemplate(&body, sc.tmpl, data); err != nil {
		body.Reset()
		_ = s.templates.ExecuteTemplate(&body, "render-failed", data)
	}
	if len(actions) > 0 {
		if err := s.templates.ExecuteTemplate(&actionHTML, "actions", actions); err != nil {
			actionHTML.Reset()
		}
	}
	// One vertical rhythm for every screen: the shell's stack owns spacing.
	var framed bytes.Buffer
	framed.WriteString(`<div class="gas-stack">`)
	if data.Frame.Incident != nil {
		_ = s.templates.ExecuteTemplate(&framed, "incident-context", data)
	}
	_ = s.templates.ExecuteTemplate(&framed, "time-trust", data)
	framed.Write(body.Bytes())
	framed.WriteString(`</div>`)

	summary := sc.summary
	if data.Frame.Incident != nil && sc.incident {
		viewing := data.Compartment
		if viewing == "" {
			viewing = "GENERAL"
		}
		if summary != "" {
			summary += " "
		}
		summary += "Viewing " + viewing + "."
	}
	docTitle := sc.title + " · CRAB"
	if data.Frame.Incident != nil {
		docTitle = sc.title + " · " + data.Frame.Incident.ID + " · CRAB"
	}
	userLabel := ""
	if data.Frame.HumanID != "" {
		userLabel = data.Frame.HumanID + " · " + data.Frame.Role
		if data.Frame.Custodian {
			userLabel += " · Custodian"
		}
	}
	var nav []shell.NavGroup
	appStatus := ""
	if data.Frame.HumanID != "" {
		nav = s.navigation(data.Frame, sc.nav)
		appStatus = statusLine(data.Frame)
	}
	p := shell.Page{
		DocumentTitle: docTitle,
		App:           shell.AppHeader{Name: "CRAB", Context: headerContext(data.Frame), Status: appStatus, UserLabel: userLabel},
		Navigation:    nav,
		Title:         sc.title,
		Summary:       summary,
		Actions:       template.HTML(actionHTML.String()),
		Workspace:     shell.Workspace{Mode: shell.WorkspaceSingle, Content: template.HTML(framed.String())},
		PWA:           &shell.PWAHead{ManifestURL: "/manifest.webmanifest", RegistrationURL: shellAssetBase + "/pwa.js", ServiceWorkerURL: "/service-worker.js"},
	}
	if data.Frame.HumanID == "" && sc.nav == "offline" {
		p.App.Status = "Offline"
		p.PWA = nil
	}
	var out bytes.Buffer
	if err := s.shell.Render(&out, p); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// show is the GET helper: session, frame, load, render.
func (s *Server) show(sc screen) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := sessionToken(r)
		if token == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.render(w, r, token, sc, http.StatusOK, pageData{Notice: messages[r.URL.Query().Get("m")], Open: r.URL.Query().Get("open")})
	}
}

// render resolves the frame and loads screen data. A load refusal renders a
// calm in-frame notice without naming an incident the user cannot see.
func (s *Server) render(w http.ResponseWriter, r *http.Request, token string, sc screen, status int, data pageData) {
	incidentID := ""
	if sc.incident {
		incidentID = r.PathValue("incidentID")
	}
	frame, ok, err := s.runtime.Frame(r.Context(), token, incidentID)
	if err != nil || !ok {
		s.clearSession(w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	data.Frame = frame
	if sc.incident {
		data.IncidentID = incidentID
		data.Compartment = compartmentParam(r)
		if frame.Incident == nil {
			s.page(w, http.StatusNotFound, screen{nav: sc.nav, title: "Not available", tmpl: "refused"}, pageData{Frame: frame, Error: "This incident is not available to you."}, nil)
			return
		}
	}
	if sc.load != nil {
		value, actions, err := sc.load(r, token, frame)
		if err != nil {
			s.page(w, refusalStatus(err), screen{nav: sc.nav, title: sc.title, tmpl: "refused", incident: sc.incident}, pageData{Frame: frame, Error: refusalText(err), IncidentID: data.IncidentID, Compartment: data.Compartment}, nil)
			return
		}
		data.Data = value
		s.page(w, status, sc, data, actions)
		return
	}
	s.page(w, status, sc, data, nil)
}

// post is the mutation helper: session, same-origin, small form, call, PRG.
// A refusal re-renders the owning screen with an error summary and the input.
func (s *Server) post(sc screen, okURL func(r *http.Request) string, msg string, do func(r *http.Request, token string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := sessionToken(r)
		if token == "" {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if !sameOrigin(r) {
			http.Error(w, "request origin refused", http.StatusForbidden)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			if err := parseSmallForm(w, r); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if err := do(r, token); err != nil {
			form := r.PostForm
			if r.MultipartForm != nil {
				form = url.Values(r.MultipartForm.Value)
			}
			s.render(w, r, token, sc, refusalStatus(err), pageData{Error: refusalText(err), Form: form, Open: r.FormValue("_form")})
			return
		}
		target := okURL(r)
		sep := "?"
		if strings.Contains(target, "?") {
			sep = "&"
		}
		http.Redirect(w, r, target+sep+"m="+url.QueryEscape(msg), http.StatusSeeOther)
	}
}

type fieldError struct{ msg string }

func (e fieldError) Error() string { return e.msg }

func refusalStatus(err error) int {
	var fe fieldError
	if errors.As(err, &fe) {
		return http.StatusUnprocessableEntity
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "denied") || strings.Contains(text, "authoris") || strings.Contains(text, "authentication") {
		return http.StatusForbidden
	}
	if strings.Contains(text, "not found") {
		return http.StatusNotFound
	}
	return http.StatusUnprocessableEntity
}

func refusalText(err error) string {
	text := strings.TrimSpace(err.Error())
	if text == "" {
		return "The request was refused."
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

// ---- request helpers -------------------------------------------------------

func incidentURL(r *http.Request, suffix string) string {
	return "/incident/" + url.PathEscape(r.PathValue("incidentID")) + suffix
}

func compartmentParam(r *http.Request) string {
	value := strings.TrimSpace(r.URL.Query().Get("compartment"))
	if value == "" {
		value = strings.TrimSpace(r.FormValue("compartment"))
	}
	if value == "" {
		value = "GENERAL"
	}
	return value
}

func formValue(r *http.Request, name string) string { return strings.TrimSpace(r.FormValue(name)) }

// parseLocalTime parses an HTML datetime-local value as UTC (the display zone
// labelled on every time field until a deployment display zone is decided).
func parseLocalTime(value, field string, required bool) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return nil, fieldError{field + " is required"}
		}
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", time.RFC3339} {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			parsed = parsed.UTC()
			return &parsed, nil
		}
	}
	return nil, fieldError{field + " is not a valid date and time"}
}

func lines(raw string) []string {
	values := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func parseSmallForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		return errors.New("invalid form")
	}
	return nil
}

func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (s *Server) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

func sameOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return loopbackRequest(r)
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	return equivalentLoopbackHosts(u.Host, r.Host)
}

func equivalentLoopbackHosts(originHost, requestHost string) bool {
	originName, originPort, err := net.SplitHostPort(originHost)
	if err != nil {
		return false
	}
	requestName, requestPort, err := net.SplitHostPort(requestHost)
	if err != nil || originPort != requestPort {
		return false
	}
	return loopbackHost(originName) && loopbackHost(requestName)
}

func loopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func loopbackRequest(r *http.Request) bool {
	if r.RemoteAddr == "" {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ---- template functions ----------------------------------------------------

var templateFuncs = template.FuncMap{
	"when":      formatTime,
	"whenPtr":   formatTimePtr,
	"size":      formatBytes,
	"modeLabel": modeLabel,
	"upper":     strings.ToUpper,
	"join":      strings.Join,
	"incidentURL": func(id string, suffix string) string {
		return "/incident/" + url.PathEscape(id) + suffix
	},
	"pathEscape": url.PathEscape,
	"statusBadge": func(status string) string {
		switch strings.ToLower(status) {
		case "pass", "delivered", "complete", "confirmed", "firm", "verified", "met", "live":
			return "success"
		case "warn", "provisional", "suppressed", "in_progress", "not-configured", "exercise", "skipped":
			return "warning"
		case "fail", "failed", "overdue", "rejected", "not met":
			return "danger"
		}
		return "info"
	},
	"actionStatus": func(status string) string { return strings.ReplaceAll(status, "_", " ") },
	"humanType":    humanRecordType,
	"checked": func(form url.Values, name, value string) bool {
		return form != nil && form.Get(name) == value
	},
	"open": func(current, name string) bool { return current == name },
	"sortedKeys": func(m map[string][]domain.NotificationOutcomeView) []string {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	},
	"outcomes": func(m map[string][]domain.NotificationOutcomeView, id string) []domain.NotificationOutcomeView {
		return m[id]
	},
	"inc": func(n int) int { return n + 1 },
	"paper": func(action, id, label string) map[string]string {
		return map[string]string{"Action": action, "ID": id, "Label": label}
	},
	"sameTime": func(a, b time.Time) bool { return a.Equal(b) },
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2 Jan 2006 15:04 UTC")
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return formatTime(*t)
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}

var recordTypeLabels = map[string]string{
	"incident.declaration":                 "Incident declaration",
	"incident.fact":                        "Fact",
	"incident.uncertainty":                 "Unknown or hypothesis",
	"incident.decision":                    "Decision",
	"incident.action":                      "Action",
	"incident.operational_role_assignment": "Operational role",
	"incident.severity_change":             "Severity change",
	"incident.scope_assessment":            "Scope assessment",
	"incident.handover":                    "Handover",
	"incident.resource_request":            "Resource request",
	"incident.recovery_update":             "Recovery update",
	"incident.stand_down":                  "Stand-down",
	"incident.pir":                         "Post-Incident Review",
	"incident.retention":                   "Retention and hold",
	"incident.closure":                     "Incident closure",
	"incident.paper_transcription":         "Paper transcription",
	"incident.communication":               "Communication issue",
	"incident.communication_draft":         "Communication draft",
	"incident.communication_cadence":       "Report cadence",
	"incident.incoming_enquiry":            "Incoming enquiry",
	"incident.notification_outcome":        "Notification outcome",
	"incident.evidence":                    "Evidence",
	"incident.evidence_handling":           "Evidence handling",
	"incident.obligation_trigger":          "Obligation clock",
	"incident.obligation_change":           "Obligation change",
	"system.obligation_definition":         "Obligation definition",
	"intake.first_report":                  "First report",
	"intake.first_report_disposition":      "First report triage",
	"security.incident_membership":         "Incident access",
	"security.compartment_membership":      "Compartment read-in",
	"security.incident_sponsor":            "Incident sponsor",
	"security.compartment_controller":      "Compartment controller",
	"security.isolate_mode":                "Isolate mode",
	"security.custodian_designation":       "Custodian designation",
	"security.role_eligibility":            "Role eligibility",
	"security.output_permission":           "Output permission",
}

// humanRecordType turns an internal record type into operator language.
func humanRecordType(recordType string) string {
	if label, ok := recordTypeLabels[recordType]; ok {
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

// writeInlinePDF shows a CRAB-generated PDF in the browser's viewer (T-197).
// The HTML content policy is replaced by one suited to a PDF body: no
// framing, no sniffing, no caching. Evidence attachments never use this.
func writeInlinePDF(w http.ResponseWriter, filename string, body []byte) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+filename+`"`)
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = io.Copy(w, bytes.NewReader(body))
}

// writeAttachment streams bytes as a no-store attachment.
func writeAttachment(w http.ResponseWriter, filename, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = io.Copy(w, bytes.NewReader(body))
}
