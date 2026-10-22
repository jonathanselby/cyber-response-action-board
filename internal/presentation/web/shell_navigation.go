package web

import (
	"net/url"

	appshell "github.com/jonathanselby/go-app-shell/shell"
)

func shellNavigation(incidentID, current string) []appshell.NavGroup {
	global := []appshell.NavItem{
		{Label: "Incidents", URL: "/", Current: current == "incidents"},
		{Label: "Control", URL: "/control", Current: current == "control"},
		{Label: "Readiness", URL: "/readiness", Current: current == "readiness"},
		{Label: "Access", URL: "/access", Current: current == "access" && incidentID == ""},
		{Label: "Obligation definitions", URL: "/obligations", Current: current == "obligation-definitions"},
		{Label: "First-report intake", URL: "/intake", Current: current == "intake"},
	}
	groups := []appshell.NavGroup{{Label: "CRAB", Items: global}}
	if incidentID == "" {
		return groups
	}
	escaped := url.PathEscape(incidentID)
	incident := []appshell.NavItem{
		{Label: "Board", URL: "/incident/" + escaped, Current: current == "board"},
		{Label: "Operations", URL: "/incident/" + escaped + "/operations", Current: current == "operations"},
		{Label: "Access", URL: "/incident/" + escaped + "/access", Current: current == "access"},
		{Label: "Evidence", URL: "/incident/" + escaped + "/evidence", Current: current == "evidence"},
		{Label: "Clocks", URL: "/incident/" + escaped + "/obligations", Current: current == "obligations"},
		{Label: "Communications", URL: "/incident/" + escaped + "/communications", Current: current == "communications"},
		{Label: "Incident control", URL: "/incident/" + escaped + "/control", Current: current == "incident-control"},
	}
	return append(groups, appshell.NavGroup{Label: incidentID, Items: incident})
}
