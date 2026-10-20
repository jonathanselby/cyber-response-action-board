package mcp

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

type fakeRuntime struct {
	factToken, factIncident, factStatement, factSource string
}

func (f *fakeRuntime) DeclareIncident(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (f *fakeRuntime) RecordFact(_ context.Context, token, incidentID, statement, source string) error {
	f.factToken, f.factIncident, f.factStatement, f.factSource = token, incidentID, statement, source
	return nil
}
func (f *fakeRuntime) ProposeAction(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (f *fakeRuntime) Decide(context.Context, string, string, string, string, string, string) error {
	return nil
}

func TestCurrentDiscoveryAndToolList(t *testing.T) {
	in := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"server/discover\",\"params\":{\"_meta\":{\"io.modelcontextprotocol/protocolVersion\":\"2026-07-28\"}}}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\",\"params\":{\"_meta\":{\"io.modelcontextprotocol/protocolVersion\":\"2026-07-28\"}}}\n")
	var out bytes.Buffer
	if err := (Server{Runtime: &fakeRuntime{}, Token: "operator-session", Version: "0.1.0"}).Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"2026-07-28", "incident_declare", "fact_record", "action_propose", "decision_decide"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}

func TestLegacyInitializeRemainsCompatible(t *testing.T) {
	in := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-11-25\"}}\n")
	var out bytes.Buffer
	if err := (Server{Runtime: &fakeRuntime{}, Token: "operator-session", Version: "0.1.0"}).Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "2025-11-25") || !strings.Contains(out.String(), "serverInfo") {
		t.Fatalf("legacy initialize = %s", out.String())
	}
}

func TestToolCallUsesAuthenticatedSharedRuntime(t *testing.T) {
	runtime := &fakeRuntime{}
	in := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":7,\"method\":\"tools/call\",\"params\":{\"name\":\"fact_record\",\"arguments\":{\"incident_id\":\"INC-1\",\"statement\":\"Service unavailable\",\"source\":\"operator\"}}}\n")
	var out bytes.Buffer
	if err := (Server{Runtime: runtime, Token: "operator-session", Version: "0.1.0"}).Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}
	if runtime.factToken != "operator-session" || runtime.factIncident != "INC-1" || runtime.factStatement != "Service unavailable" || runtime.factSource != "operator" {
		t.Fatalf("runtime call = %+v", runtime)
	}
	if !strings.Contains(out.String(), "immutable incident history") || strings.Contains(out.String(), "\"isError\":true") {
		t.Fatalf("tool result = %s", out.String())
	}
}

func TestToolFailureDoesNotLeakInternalError(t *testing.T) {
	in := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":9,\"method\":\"tools/call\",\"params\":{\"name\":\"unknown\",\"arguments\":{}}}\n")
	var out bytes.Buffer
	if err := (Server{Runtime: &fakeRuntime{}, Token: "operator-session", Version: "0.1.0"}).Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "CRAB refused the request") || !strings.Contains(out.String(), "\"isError\":true") || strings.Contains(out.String(), "unknown tool") {
		t.Fatalf("tool failure = %s", out.String())
	}
}
