package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	ProtocolCurrent = "2026-07-28"
	ProtocolLegacy  = "2025-11-25"
	maxMessageBytes = 256 << 10
)

type Runtime interface {
	DeclareIncident(context.Context, string, string, string, string, string, string) error
	RecordFact(context.Context, string, string, string, string) error
	ProposeAction(context.Context, string, string, string, string, string, string) error
	Decide(context.Context, string, string, string, string, string, string) error
}

type Server struct {
	Runtime Runtime
	Token   string
	Version string
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	if s.Runtime == nil || strings.TrimSpace(s.Token) == "" || strings.TrimSpace(s.Version) == "" {
		return errors.New("MCP server requires runtime, authenticated operator session and version")
	}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), maxMessageBytes)
	enc := json.NewEncoder(out)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			if err := enc.Encode(response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}}); err != nil {
				return err
			}
			continue
		}
		if req.JSONRPC != "2.0" || req.Method == "" {
			if err := enc.Encode(response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32600, Message: "invalid request"}}); err != nil {
				return err
			}
			continue
		}
		res, send := s.handle(ctx, req)
		if !send {
			continue
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s Server) handle(ctx context.Context, req request) (response, bool) {
	base := response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "notifications/initialized", "notifications/cancelled":
		return response{}, false
	case "server/discover":
		base.Result = map[string]any{
			"resultType":        "complete",
			"supportedVersions": []string{ProtocolCurrent, ProtocolLegacy, "2025-06-18"},
			"capabilities":      map[string]any{"tools": map[string]any{"listChanged": false}},
			"instructions":      "CRAB tools use the authenticated local operator session and the same authorisation, maker-checker and ledger path as the human UI.",
			"_meta":             map[string]any{"io.modelcontextprotocol/serverInfo": map[string]any{"name": "crab", "version": s.Version}},
			"ttlMs":             60000,
			"cacheScope":        "private",
		}
		return base, true
	case "initialize":
		base.Result = map[string]any{
			"protocolVersion": ProtocolLegacy,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "crab", "version": s.Version},
			"instructions":    "CRAB mutations remain human-accountable and maker-checker governed.",
		}
		return base, true
	case "tools/list":
		base.Result = map[string]any{"resultType": "complete", "tools": tools(), "ttlMs": 60000, "cacheScope": "private"}
		return base, true
	case "tools/call":
		var params callParams
		if err := json.Unmarshal(req.Params, &params); err != nil || strings.TrimSpace(params.Name) == "" {
			base.Error = &rpcError{Code: -32602, Message: "invalid tool call"}
			return base, true
		}
		text, err := s.call(ctx, params)
		base.Result = map[string]any{"resultType": "complete", "content": []map[string]string{{"type": "text", "text": text}}, "isError": err != nil}
		if err != nil {
			base.Result = map[string]any{"resultType": "complete", "content": []map[string]string{{"type": "text", "text": "CRAB refused the request"}}, "isError": true}
		}
		return base, true
	default:
		base.Error = &rpcError{Code: -32601, Message: "method not found"}
		return base, true
	}
}

func (s Server) call(ctx context.Context, params callParams) (string, error) {
	switch params.Name {
	case "incident_declare":
		var a struct {
			IncidentID string `json:"incident_id"`
			Mode       string `json:"mode"`
			Title      string `json:"title"`
			Summary    string `json:"summary"`
			Severity   string `json:"severity"`
		}
		if err := strictArguments(params.Arguments, &a); err != nil {
			return "", err
		}
		if err := s.Runtime.DeclareIncident(ctx, s.Token, a.IncidentID, a.Mode, a.Title, a.Summary, a.Severity); err != nil {
			return "", err
		}
		return "incident declaration proposed; a different eligible human must confirm it", nil
	case "fact_record":
		var a struct {
			IncidentID string `json:"incident_id"`
			Statement  string `json:"statement"`
			Source     string `json:"source"`
		}
		if err := strictArguments(params.Arguments, &a); err != nil {
			return "", err
		}
		if err := s.Runtime.RecordFact(ctx, s.Token, a.IncidentID, a.Statement, a.Source); err != nil {
			return "", err
		}
		return "fact recorded as immutable incident history", nil
	case "action_propose":
		var a struct {
			IncidentID string `json:"incident_id"`
			Title      string `json:"title"`
			Owner      string `json:"owner"`
			Priority   string `json:"priority"`
			SourceRef  string `json:"source_ref"`
		}
		if err := strictArguments(params.Arguments, &a); err != nil {
			return "", err
		}
		if err := s.Runtime.ProposeAction(ctx, s.Token, a.IncidentID, a.Title, a.Owner, a.Priority, a.SourceRef); err != nil {
			return "", err
		}
		return "action proposed; it is not effective until confirmation", nil
	case "decision_decide":
		var a struct {
			IncidentID string `json:"incident_id"`
			RecordType string `json:"record_type"`
			RecordID   string `json:"record_id"`
			Version    string `json:"version"`
			Outcome    string `json:"outcome"`
		}
		if err := strictArguments(params.Arguments, &a); err != nil {
			return "", err
		}
		if err := s.Runtime.Decide(ctx, s.Token, a.IncidentID, a.RecordType, a.RecordID, a.Version, a.Outcome); err != nil {
			return "", err
		}
		return fmt.Sprintf("proposal %s", a.Outcome), nil
	default:
		return "", errors.New("unknown tool")
	}
}

func strictArguments(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return errors.New("tool arguments are required")
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	return nil
}

func tools() []tool {
	object := func(required []string, properties map[string]any) map[string]any {
		return map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}
	}
	str := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description, "minLength": 1}
	}
	return []tool{
		{Name: "incident_declare", Description: "Propose a CRAB incident declaration through the governed maker-checker path.", InputSchema: object([]string{"incident_id", "mode", "title", "summary", "severity"}, map[string]any{"incident_id": str("incident identifier"), "mode": map[string]any{"type": "string", "enum": []string{"live", "exercise"}}, "title": str("incident title"), "summary": str("current situation summary"), "severity": str("human-assessed severity")})},
		{Name: "fact_record", Description: "Record an observed fact as immutable CRAB incident history.", InputSchema: object([]string{"incident_id", "statement", "source"}, map[string]any{"incident_id": str("incident identifier"), "statement": str("fact statement"), "source": str("source or provenance")})},
		{Name: "action_propose", Description: "Propose an incident action; it remains ineffective until maker-checker confirmation.", InputSchema: object([]string{"incident_id", "title", "owner", "priority", "source_ref"}, map[string]any{"incident_id": str("incident identifier"), "title": str("action"), "owner": str("named human owner"), "priority": str("priority"), "source_ref": str("source reference")})},
		{Name: "decision_decide", Description: "Confirm or reject an existing exact-version CRAB proposal as the authenticated confirmer.", InputSchema: object([]string{"incident_id", "record_type", "record_id", "version", "outcome"}, map[string]any{"incident_id": str("incident identifier"), "record_type": str("semantic record type"), "record_id": str("proposal record identifier"), "version": str("exact proposal version"), "outcome": map[string]any{"type": "string", "enum": []string{"confirmed", "rejected"}}})},
	}
}
