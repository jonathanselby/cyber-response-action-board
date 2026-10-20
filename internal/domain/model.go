package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ModelLane string

const (
	ModelLaneSimple   ModelLane = "simple"
	ModelLaneStandard ModelLane = "standard"
	ModelLaneComplex  ModelLane = "complex"

	ModelInputRulePolicyGate = "managed_if_policy_allows"
	MaxModelDraftBytes       = 64 << 10
)

func (l ModelLane) Valid() bool {
	return l == ModelLaneSimple || l == ModelLaneStandard || l == ModelLaneComplex
}

type ModelDraftSpec struct {
	ID               string      `json:"id"`
	Purpose          string      `json:"purpose"`
	PromptVersion    string      `json:"prompt_version"`
	AllowedLanes     []ModelLane `json:"allowed_lanes"`
	InputRule        string      `json:"input_rule"`
	OutputType       string      `json:"output_type"`
	RequireCitations bool        `json:"require_citations"`
	MaxOutputBytes   int         `json:"max_output_bytes"`
}

type ModelLanePolicy struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Managed  bool   `json:"managed"`
}

type ModelProviderResult struct {
	Output       string `json:"-"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}

type ModelTrace struct {
	Timestamp     time.Time `json:"timestamp"`
	RequestID     string    `json:"request_id"`
	SpecID        string    `json:"spec_id"`
	PromptVersion string    `json:"prompt_version"`
	Lane          ModelLane `json:"lane"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	Managed       bool      `json:"managed"`
	PromptSHA256  string    `json:"prompt_sha256"`
	CitationIDs   []string  `json:"citation_ids,omitempty"`
	DurationMS    int64     `json:"duration_ms"`
	Success       bool      `json:"success"`
	Error         string    `json:"error,omitempty"`
	InputTokens   int       `json:"input_tokens,omitempty"`
	OutputTokens  int       `json:"output_tokens,omitempty"`
}

type ModelDraft struct {
	SpecID        string              `json:"spec_id"`
	PromptVersion string              `json:"prompt_version"`
	OutputType    string              `json:"output_type"`
	Content       string              `json:"content,omitempty"`
	Untrusted     bool                `json:"untrusted"`
	Validated     bool                `json:"validated"`
	Failure       string              `json:"failure,omitempty"`
	Citations     []RetrievalCitation `json:"citations,omitempty"`
	Trace         ModelTrace          `json:"trace"`
}

func ModelDraftSpecByID(id string) (ModelDraftSpec, bool) {
	specs := []ModelDraftSpec{
		{ID: "retrieval-summary", Purpose: "Summarise authorised retrieved passages without adding unsupported claims.", PromptVersion: "retrieval-summary/v1", AllowedLanes: []ModelLane{ModelLaneSimple, ModelLaneStandard}, InputRule: ModelInputRulePolicyGate, OutputType: "untrusted_summary", RequireCitations: true, MaxOutputBytes: MaxModelDraftBytes},
		{ID: "situation-report-draft", Purpose: "Draft a situation report from authorised supplied sources for later human review.", PromptVersion: "situation-report-draft/v1", AllowedLanes: []ModelLane{ModelLaneStandard, ModelLaneComplex}, InputRule: ModelInputRulePolicyGate, OutputType: "untrusted_situation_report", RequireCitations: true, MaxOutputBytes: MaxModelDraftBytes},
		{ID: "board-update-draft", Purpose: "Draft a concise board update from authorised supplied sources for later human review.", PromptVersion: "board-update-draft/v1", AllowedLanes: []ModelLane{ModelLaneStandard}, InputRule: ModelInputRulePolicyGate, OutputType: "untrusted_board_update", RequireCitations: true, MaxOutputBytes: MaxModelDraftBytes},
		{ID: "notification-language-draft", Purpose: "Draft notification language from authorised supplied sources without issuing it.", PromptVersion: "notification-language-draft/v1", AllowedLanes: []ModelLane{ModelLaneSimple, ModelLaneStandard}, InputRule: ModelInputRulePolicyGate, OutputType: "untrusted_notification_language", RequireCitations: true, MaxOutputBytes: MaxModelDraftBytes},
		{ID: "pir-summary-draft", Purpose: "Draft a post-incident-review summary from authorised supplied sources for later human review.", PromptVersion: "pir-summary-draft/v1", AllowedLanes: []ModelLane{ModelLaneComplex}, InputRule: ModelInputRulePolicyGate, OutputType: "untrusted_pir_summary", RequireCitations: true, MaxOutputBytes: MaxModelDraftBytes},
	}
	for _, spec := range specs {
		if spec.ID == id {
			return spec, true
		}
	}
	return ModelDraftSpec{}, false
}

func ModelSpecAllowsLane(spec ModelDraftSpec, lane ModelLane) bool {
	for _, allowed := range spec.AllowedLanes {
		if allowed == lane {
			return true
		}
	}
	return false
}

func BuildModelPrompt(spec ModelDraftSpec, instruction string, citations []RetrievalCitation) (string, string, error) {
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		return "", "", errors.New("model instruction is required")
	}
	if spec.ID == "" || spec.PromptVersion == "" || spec.Purpose == "" || spec.OutputType == "" {
		return "", "", errors.New("model draft spec is incomplete")
	}
	if spec.RequireCitations && len(citations) == 0 {
		return "", "", errors.New("model draft spec requires authorised citations")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "CRAB DRAFT SPEC: %s\nPROMPT VERSION: %s\nPURPOSE: %s\nOUTPUT TYPE: %s\n\n", spec.ID, spec.PromptVersion, spec.Purpose, spec.OutputType)
	b.WriteString("RULES:\n- This is untrusted drafting assistance, never an approval or effective state change.\n- Use only the supplied authorised sources.\n- Do not invent facts, decisions, approvals, legal conclusions or citations.\n- If the sources do not support a requested statement, say that it is unsupported.\n\n")
	fmt.Fprintf(&b, "TASK:\n%s\n\nAUTHORISED SOURCES:\n", instruction)
	for _, citation := range citations {
		fmt.Fprintf(&b, "[%s] %s | %s\n%s\n\n", citation.CitationID, citation.Title, citation.Locator, citation.Passage)
	}
	prompt := b.String()
	digest := sha256.Sum256([]byte(prompt))
	return prompt, hex.EncodeToString(digest[:]), nil
}

func ValidateModelOutput(spec ModelDraftSpec, output string) error {
	if strings.TrimSpace(output) == "" {
		return errors.New("model returned empty draft")
	}
	limit := spec.MaxOutputBytes
	if limit <= 0 {
		limit = MaxModelDraftBytes
	}
	if len(output) > limit {
		return errors.New("model draft exceeds configured maximum")
	}
	if strings.ContainsRune(output, '\x00') {
		return errors.New("model draft contains NUL byte")
	}
	return nil
}

func ModelCitationIDs(citations []RetrievalCitation) []string {
	ids := make([]string, 0, len(citations))
	for _, citation := range citations {
		if citation.CitationID != "" {
			ids = append(ids, citation.CitationID)
		}
	}
	return ids
}
