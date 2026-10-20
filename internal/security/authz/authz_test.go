package authz

import "testing"

func principal(role Role) Principal {
	return Principal{
		Authenticated: true,
		HumanID:       "alice",
		Role:          role,
		Incidents:     map[string]bool{"INC-A": true},
		Compartments:  map[string]map[string]bool{"INC-A": {"GENERAL": true, "LEGAL": true}},
		Outputs:       map[Action]bool{},
	}
}

func TestMajorHasNoUniversalIncidentVisibility(t *testing.T) {
	p := principal(RoleMajor)
	if d := Authorise(p, ActionRead, ObjectScope{IncidentID: "INC-B", Compartment: "GENERAL"}); d.Allowed {
		t.Fatalf("cross-incident read allowed: %+v", d)
	}
}

func TestRestrictedCompartmentRequiresExplicitMembership(t *testing.T) {
	p := principal(RoleMajor)
	delete(p.Compartments["INC-A"], "LEGAL")
	if d := Authorise(p, ActionRead, ObjectScope{IncidentID: "INC-A", Compartment: "LEGAL"}); d.Allowed {
		t.Fatalf("restricted compartment read allowed: %+v", d)
	}
}

func TestOutputPermissionIsSeparateFromRead(t *testing.T) {
	p := principal(RoleObserver)
	scope := ObjectScope{IncidentID: "INC-A", Compartment: "GENERAL"}
	if d := Authorise(p, ActionRead, scope); !d.Allowed {
		t.Fatalf("ordinary authorised read denied: %+v", d)
	}
	if d := Authorise(p, ActionExport, scope); d.Allowed {
		t.Fatalf("export inherited ordinary read: %+v", d)
	}
	p.Outputs[ActionExport] = true
	if d := Authorise(p, ActionExport, scope); !d.Allowed {
		t.Fatalf("separately authorised export denied: %+v", d)
	}
}

func TestCustodianDoesNotGainIncidentContent(t *testing.T) {
	p := principal(RoleObserver)
	p.Custodian = true
	delete(p.Incidents, "INC-A")
	if d := Authorise(p, ActionRead, ObjectScope{IncidentID: "INC-A", Compartment: "GENERAL"}); d.Allowed {
		t.Fatalf("custodian content read allowed: %+v", d)
	}
	if d := Authorise(p, ActionAdminIdentityMetadata, ObjectScope{SystemMetadata: true}); !d.Allowed {
		t.Fatalf("custodian metadata administration denied: %+v", d)
	}
}

func TestUnauthenticatedAndUnknownActionDeny(t *testing.T) {
	p := principal(RoleMajor)
	p.Authenticated = false
	if d := Authorise(p, ActionRead, ObjectScope{IncidentID: "INC-A"}); d.Allowed {
		t.Fatalf("unauthenticated principal allowed: %+v", d)
	}
	p.Authenticated = true
	if d := Authorise(p, Action("invented"), ObjectScope{IncidentID: "INC-A"}); d.Allowed {
		t.Fatalf("unknown action allowed: %+v", d)
	}
}
