package web

import (
	"net/http"
	"strings"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

var organisationScreen = screen{nav: "organisation", title: "Organisation", summary: "Governed policy every incident uses: severity scale and emergency information.", tmpl: "organisation"}

func (s *Server) registerOrganisationRoutes() {
	sc := organisationScreen
	sc.load = func(r *http.Request, token string, _ FrameView) (any, []Action, error) {
		page, err := s.runtime.Organisation(r.Context(), token)
		var actions []Action
		if err == nil && page.CanPropose {
			actions = []Action{{Label: "Propose emergency information", URL: "?open=emergency#emergency", Primary: true}}
		}
		return page, actions, err
	}
	back := func(*http.Request) string { return "/organisation" }
	s.mux.HandleFunc("GET /organisation", s.show(sc))
	s.mux.HandleFunc("POST /organisation/severity", s.post(sc, back, "proposed", func(r *http.Request, token string) error {
		return s.runtime.ProposeSeverityScale(r.Context(), token, lines(r.FormValue("levels")))
	}))
	s.mux.HandleFunc("POST /organisation/emergency", s.post(sc, back, "proposed", func(r *http.Request, token string) error {
		value, err := parseEmergencyForm(r)
		if err != nil {
			return err
		}
		return s.runtime.ProposeEmergencyConfiguration(r.Context(), token, value)
	}))
}

// splitRow splits one "a | b | c" line into exactly n trimmed fields.
func splitRow(line string, n int) []string {
	parts := strings.Split(line, "|")
	out := make([]string, n)
	for i := 0; i < n && i < len(parts); i++ {
		out[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) > n {
		out[n-1] = strings.TrimSpace(strings.Join(parts[n-1:], "|"))
	}
	return out
}

// parseEmergencyForm reads the plain-text emergency form. Each list is one
// entry per line with fields separated by "|"; procedures start with "## ".
func parseEmergencyForm(r *http.Request) (domain.EmergencyConfiguration, error) {
	c := domain.EmergencyConfiguration{Organisation: formValue(r, "organisation")}
	for _, line := range lines(r.FormValue("contacts")) {
		f := splitRow(line, 5)
		c.Contacts = append(c.Contacts, domain.EmergencyContact{Name: f[0], Role: f[1], Primary: f[2], Alternative: f[3], Notes: f[4]})
	}
	for _, line := range lines(r.FormValue("parties")) {
		f := splitRow(line, 5)
		c.Parties = append(c.Parties, domain.ExternalParty{Category: strings.ToLower(f[0]), Name: f[1], Primary: f[2], Alternative: f[3], Notes: f[4]})
	}
	c.Escalation = lines(r.FormValue("escalation"))
	for _, line := range lines(r.FormValue("role_cards")) {
		f := splitRow(line, 3)
		c.RoleCards = append(c.RoleCards, domain.RoleCard{Title: f[0], Responsibilities: f[1], Deputy: f[2]})
	}
	var current *domain.Procedure
	for _, raw := range strings.Split(strings.ReplaceAll(r.FormValue("procedures"), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(raw), "## ") {
			c.Procedures = append(c.Procedures, domain.Procedure{Title: strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "## "))})
			current = &c.Procedures[len(c.Procedures)-1]
			continue
		}
		if current == nil {
			if strings.TrimSpace(raw) != "" {
				return c, fieldError{"Each procedure must start with a line beginning \"## \" and its title"}
			}
			continue
		}
		if current.Body != "" || strings.TrimSpace(raw) != "" {
			current.Body += raw + "\n"
		}
	}
	for i := range c.Procedures {
		c.Procedures[i].Body = strings.TrimSpace(c.Procedures[i].Body)
	}
	if err := domain.ValidateEmergencyConfiguration(c); err != nil {
		return c, fieldError{err.Error()}
	}
	return c, nil
}

// emergencyText renders configuration back into the editable form.
func emergencyText(c *domain.EmergencyConfiguration, field string) string {
	if c == nil {
		return ""
	}
	var b strings.Builder
	row := func(values ...string) {
		for len(values) > 0 && values[len(values)-1] == "" {
			values = values[:len(values)-1]
		}
		b.WriteString(strings.Join(values, " | "))
		b.WriteString("\n")
	}
	switch field {
	case "contacts":
		for _, v := range c.Contacts {
			row(v.Name, v.Role, v.Primary, v.Alternative, v.Notes)
		}
	case "parties":
		for _, v := range c.Parties {
			row(v.Category, v.Name, v.Primary, v.Alternative, v.Notes)
		}
	case "escalation":
		for _, v := range c.Escalation {
			row(v)
		}
	case "role_cards":
		for _, v := range c.RoleCards {
			row(v.Title, v.Responsibilities, v.Deputy)
		}
	case "procedures":
		for _, v := range c.Procedures {
			b.WriteString("## " + v.Title + "\n" + v.Body + "\n\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
