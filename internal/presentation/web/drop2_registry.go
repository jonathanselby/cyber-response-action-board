package web

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
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
	Error      string
	Data       any
}

func (s *Server) renderDrop2(w http.ResponseWriter, tmpl *template.Template, page drop2Page) {
	page.Version = s.version
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, page); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
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
