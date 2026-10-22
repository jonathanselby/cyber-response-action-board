package web

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	appshell "github.com/jonathanselby/go-app-shell/shell"
)

var drop2RouteRegistrars []func(*Server)

func registerDrop2Registrar(fn func(*Server)) {
	if fn != nil {
		drop2RouteRegistrars = append(drop2RouteRegistrars, fn)
	}
}

func (s *Server) registerDrop2Routes() {
	for _, fn := range drop2RouteRegistrars {
		fn(s)
	}
}

type drop2Page struct {
	Version    string
	Title      string
	IncidentID string
	Current    string
	Summary    string
	Error      string
	Data       any
}

func (s *Server) renderSpecialist(w http.ResponseWriter, tmpl *template.Template, page drop2Page) {
	page.Version = s.version
	if page.Current == "" {
		page.Current = specialistCurrent(page.Title, page.IncidentID)
	}
	if page.Summary == "" {
		if page.IncidentID == "" {
			page.Summary = "Authorised CRAB operational view."
		} else {
			page.Summary = "Authorised operational view for " + page.IncidentID + "."
		}
	}
	app := appshell.AppHeader{Name: "CRAB", Status: "v" + s.version}
	if page.IncidentID != "" {
		app.Context = page.IncidentID
	}
	s.renderFragmentShell(w, tmpl, page, appshell.Page{
		DocumentTitle: page.Title + " · CRAB",
		App:           app,
		Navigation:    shellNavigation(page.IncidentID, page.Current),
		Title:         page.Title,
		Summary:       page.Summary,
	})
}

func (s *Server) renderFragmentShell(w http.ResponseWriter, tmpl *template.Template, data any, page appshell.Page) {
	var fragment bytes.Buffer
	if err := tmpl.Execute(&fragment, data); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	page.Workspace = appshell.Workspace{Mode: appshell.WorkspaceSingle, Content: template.HTML(fragment.String())}
	s.renderShell(w, page)
}

func specialistCurrent(title, incidentID string) string {
	switch title {
	case "Access administration", "Incident access":
		return "access"
	case "Readiness":
		return "readiness"
	case "Evidence", "Evidence detail":
		return "evidence"
	case "Obligation definitions":
		return "obligation-definitions"
	case "Obligations", "Obligations & clocks":
		if incidentID == "" {
			return "obligation-definitions"
		}
		return "obligations"
	case "Communications", "Communications & enquiries":
		return "communications"
	case "Control":
		return "control"
	case "Incident control":
		return "incident-control"
	default:
		return ""
	}
}

func requireDrop2Runtime[T any](s *Server) (T, error) {
	var zero T
	if s == nil || s.runtime == nil {
		return zero, errors.New("operator runtime unavailable")
	}
	value, ok := any(s.runtime).(T)
	if !ok {
		return zero, errors.New("requested capability is not available in this build checkpoint")
	}
	return value, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func parseOptionalRFC3339(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func parseRequiredRFC3339(value string) (time.Time, error) {
	parsed, err := parseOptionalRFC3339(value)
	if err != nil {
		return time.Time{}, err
	}
	if parsed == nil {
		return time.Time{}, errors.New("timestamp is required")
	}
	return *parsed, nil
}

func parsePositiveInt(value string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n <= 0 {
		return 0, errors.New("positive integer required")
	}
	return n, nil
}

func drop2Token(r *http.Request) (string, error) {
	token := sessionToken(r)
	if token == "" {
		return "", errors.New("authentication required")
	}
	return token, nil
}
