package domain

import (
	"encoding/hex"
	"errors"
	"mime"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

const (
	TypeEvidence         = "incident.evidence"
	TypeEvidenceHandling = "incident.evidence_handling"

	EvidencePresentationInline   = "inline"
	EvidencePresentationDownload = "download"
)

type BlobReceipt struct {
	BlobID            string `json:"blob_id"`
	SHA256            string `json:"sha256"`
	SizeBytes         int64  `json:"size_bytes"`
	OriginalFilename  string `json:"original_filename"`
	DeclaredMediaType string `json:"declared_media_type,omitempty"`
	DetectedMediaType string `json:"detected_media_type"`
}

type Evidence struct {
	BlobReceipt
	Description    string     `json:"description,omitempty"`
	Collector      string     `json:"collector"`
	Source         string     `json:"source"`
	CollectedAt    *time.Time `json:"collected_at,omitempty"`
	OccurredAt     *time.Time `json:"occurred_at,omitempty"`
	SourceLocation string     `json:"source_location,omitempty"`
	CurrentHandler string     `json:"current_handler"`
	RelatedRef     string     `json:"related_ref,omitempty"`
}

type EvidenceHandling struct {
	EvidenceID string     `json:"evidence_id"`
	Action     string     `json:"action"`
	Handler    string     `json:"handler"`
	Location   string     `json:"location,omitempty"`
	Note       string     `json:"note,omitempty"`
	OccurredAt *time.Time `json:"occurred_at,omitempty"`
}

type EvidenceView struct {
	EvidenceID  string
	IncidentID  string
	Mode        string
	Compartment string
	RecordedAt  time.Time
	TimeStatus  string
	Evidence    Evidence
}

type EvidenceHandlingView struct {
	RecordID    string
	EvidenceID  string
	IncidentID  string
	Mode        string
	Compartment string
	RecordedAt  time.Time
	TimeStatus  string
	Handling    EvidenceHandling
}

func ValidateEvidenceCaptureMetadata(filename, collector, source, currentHandler string) error {
	if strings.TrimSpace(filename) == "" {
		return errors.New("evidence filename is required")
	}
	if strings.TrimSpace(collector) == "" {
		return errors.New("evidence collector is required")
	}
	if strings.TrimSpace(source) == "" {
		return errors.New("evidence source is required")
	}
	if strings.TrimSpace(currentHandler) == "" {
		return errors.New("evidence current handler is required")
	}
	return nil
}

func ValidateEvidence(value Evidence) error {
	if err := ValidateEvidenceCaptureMetadata(value.OriginalFilename, value.Collector, value.Source, value.CurrentHandler); err != nil {
		return err
	}
	if strings.TrimSpace(value.BlobID) == "" {
		return errors.New("evidence blob id is required")
	}
	if len(value.SHA256) != 64 {
		return errors.New("evidence sha256 must be 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(value.SHA256); err != nil {
		return errors.New("evidence sha256 is invalid")
	}
	if value.SizeBytes < 0 {
		return errors.New("evidence size cannot be negative")
	}
	if strings.TrimSpace(value.DetectedMediaType) == "" {
		return errors.New("evidence detected media type is required")
	}
	return nil
}

func ValidateEvidenceHandling(value EvidenceHandling) error {
	if strings.TrimSpace(value.EvidenceID) == "" || strings.TrimSpace(value.Action) == "" || strings.TrimSpace(value.Handler) == "" {
		return errors.New("evidence handling requires evidence id, action and handler")
	}
	return nil
}

func RecordEvidence(scope Scope, recordID, maker string, now time.Time, value Evidence) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateEvidence(value); err != nil {
		return ledger.Record{}, err
	}
	return immediateRecord(scope, TypeEvidence, recordID, maker, now, value)
}

func RecordEvidenceHandling(scope Scope, recordID, maker string, now time.Time, value EvidenceHandling) (ledger.Record, error) {
	if err := validateScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateEvidenceHandling(value); err != nil {
		return ledger.Record{}, err
	}
	return immediateRecord(scope, TypeEvidenceHandling, recordID, maker, now, value)
}

func EvidencePresentationMode(declared, detected string) string {
	detectedBase := mediaBase(detected)
	if !safeInlineMedia[detectedBase] {
		return EvidencePresentationDownload
	}
	if strings.TrimSpace(declared) == "" {
		return EvidencePresentationInline
	}
	declaredBase := mediaBase(declared)
	if declaredBase == "" || declaredBase != detectedBase {
		return EvidencePresentationDownload
	}
	return EvidencePresentationInline
}

func mediaBase(value string) string {
	base, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	return strings.ToLower(base)
}

var safeInlineMedia = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
	"audio/mpeg": true,
	"audio/wav":  true,
	"audio/wave": true,
	"audio/x-wav": true,
	"audio/ogg":  true,
	"video/mp4":  true,
	"video/webm": true,
}
