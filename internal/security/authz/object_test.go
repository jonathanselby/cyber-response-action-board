package authz

import (
	"context"
	"testing"
)

type scopeMap map[string]ObjectScope

func (s scopeMap) ResolveObjectScope(_ context.Context, id string) (ObjectScope, bool, error) {
	scope, ok := s[id]
	return scope, ok, nil
}

func TestObjectIDSubstitutionCannotBypassScope(t *testing.T) {
	p := Principal{
		Authenticated: true,
		HumanID:       "major-a",
		Role:          RoleMajor,
		Incidents:     map[string]bool{"INC-A": true},
		Compartments:  map[string]map[string]bool{"INC-A": {"GENERAL": true}},
		Outputs:       map[Action]bool{},
	}
	resolver := scopeMap{
		"OBJ-A": {IncidentID: "INC-A", Compartment: "GENERAL"},
		"OBJ-B": {IncidentID: "INC-B", Compartment: "GENERAL"},
	}
	allowed, err := AuthoriseObject(context.Background(), p, ActionRead, "OBJ-A", resolver)
	if err != nil || !allowed.Allowed {
		t.Fatalf("authorised object decision=%+v err=%v", allowed, err)
	}
	denied, err := AuthoriseObject(context.Background(), p, ActionRead, "OBJ-B", resolver)
	if err != nil || denied.Allowed {
		t.Fatalf("identifier substitution decision=%+v err=%v", denied, err)
	}
	missing, err := AuthoriseObject(context.Background(), p, ActionRead, "UNKNOWN", resolver)
	if err != nil || missing.Allowed || missing.Reason != "object unavailable" {
		t.Fatalf("missing object decision=%+v err=%v", missing, err)
	}
}
