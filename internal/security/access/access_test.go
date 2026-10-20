package access

import "testing"

func TestGeneralCannotBeGrantedAsIndependentCompartment(t *testing.T) {
	_, recognised, err := Decode(RecordCompartmentMembership, []byte(`{"human_id":"h","incident_id":"i","compartment":"GENERAL","active":true}`))
	if !recognised || err == nil {
		t.Fatalf("GENERAL independent grant recognised=%v err=%v", recognised, err)
	}
}

func TestUnknownAccessRecordIsIgnored(t *testing.T) {
	_, recognised, err := Decode("incident.fact", []byte(`{"x":1}`))
	if err != nil || recognised {
		t.Fatalf("unknown record recognised=%v err=%v", recognised, err)
	}
}
