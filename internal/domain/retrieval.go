package domain

import (
	"archive/zip"
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

const (
	TypeCorpusSource      = "corpus.source"
	CorpusFormatMarkdown = "markdown"
	CorpusFormatDOCX     = "docx"
	MaxCorpusSourceBytes = int64(16 << 20)
	maxDOCXExpandedBytes = uint64(64 << 20)
	maxDOCXEntries       = 512
)

type CorpusScope struct {
	IncidentID  string
	Mode        string
	Compartment string
}

type CorpusSource struct {
	BlobReceipt
	Title  string `json:"title"`
	Format string `json:"format"`
}

type CorpusPassage struct {
	Ordinal int
	Locator string
	Text    string
	Terms   []string
}

type CorpusCandidate struct {
	SourceID    string
	Title       string
	SHA256      string
	IncidentID  string
	Compartment string
	Locator     string
	Passage     string
	Terms       []string
}

type RetrievalCitation struct {
	CitationID string `json:"citation_id"`
	SourceID   string `json:"source_id"`
	Title      string `json:"title"`
	Locator    string `json:"locator"`
	Passage    string `json:"passage"`
	Score      int    `json:"score"`
}

type RetrievalResult struct {
	Refused   bool                `json:"refused"`
	Reason    string              `json:"reason,omitempty"`
	Citations []RetrievalCitation `json:"citations,omitempty"`
}

func ValidateCorpusScope(scope CorpusScope) error {
	if strings.TrimSpace(scope.Compartment) == "" {
		return errors.New("corpus compartment is required")
	}
	switch scope.Mode {
	case "system":
		if scope.IncidentID != "" {
			return errors.New("system corpus source cannot carry incident id")
		}
	case "live", "exercise":
		if strings.TrimSpace(scope.IncidentID) == "" {
			return errors.New("incident corpus source requires incident id")
		}
	default:
		return errors.New("corpus mode must be system, live or exercise")
	}
	return nil
}

func CorpusFormatForFilename(filename string) (string, error) {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(filename))) {
	case ".md", ".markdown":
		return CorpusFormatMarkdown, nil
	case ".docx":
		return CorpusFormatDOCX, nil
	default:
		return "", errors.New("retrieval corpus accepts Markdown or DOCX only")
	}
}

func ValidateCorpusSource(value CorpusSource) error {
	if strings.TrimSpace(value.Title) == "" || strings.TrimSpace(value.BlobID) == "" || strings.TrimSpace(value.OriginalFilename) == "" {
		return errors.New("corpus source requires title, blob id and filename")
	}
	if value.SizeBytes < 0 || value.SizeBytes > MaxCorpusSourceBytes {
		return errors.New("corpus source exceeds bounded source size")
	}
	if len(value.SHA256) != 64 {
		return errors.New("corpus source sha256 must be 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(value.SHA256); err != nil {
		return errors.New("corpus source sha256 is invalid")
	}
	if strings.TrimSpace(value.DetectedMediaType) == "" {
		return errors.New("corpus source detected media type is required")
	}
	format, err := CorpusFormatForFilename(value.OriginalFilename)
	if err != nil {
		return err
	}
	if value.Format != format {
		return errors.New("corpus source format does not match filename")
	}
	return nil
}

func ProposeCorpusSource(scope CorpusScope, recordID, maker string, now time.Time, value CorpusSource) (ledger.Record, error) {
	if err := ValidateCorpusScope(scope); err != nil {
		return ledger.Record{}, err
	}
	if err := ValidateCorpusSource(value); err != nil {
		return ledger.Record{}, err
	}
	if strings.TrimSpace(recordID) == "" || strings.TrimSpace(maker) == "" || now.IsZero() {
		return ledger.Record{}, errors.New("record id, maker and time are required")
	}
	return proposedRecord(Scope{IncidentID: scope.IncidentID, Mode: scope.Mode, Compartment: scope.Compartment}, TypeCorpusSource, recordID, maker, now, value)
}

func ExtractCorpus(format string, raw []byte) ([]CorpusPassage, error) {
	if len(raw) == 0 {
		return nil, errors.New("corpus source is empty")
	}
	if int64(len(raw)) > MaxCorpusSourceBytes {
		return nil, errors.New("corpus source exceeds bounded source size")
	}
	var passages []CorpusPassage
	var err error
	switch format {
	case CorpusFormatMarkdown:
		passages, err = extractMarkdown(raw)
	case CorpusFormatDOCX:
		passages, err = extractDOCX(raw)
	default:
		return nil, errors.New("unsupported corpus format")
	}
	if err != nil {
		return nil, err
	}
	if len(passages) == 0 {
		return nil, errors.New("corpus source contains no retrievable text")
	}
	for i := range passages {
		passages[i].Ordinal = i + 1
		passages[i].Terms = LexicalTerms(passages[i].Text)
	}
	return passages, nil
}

func extractMarkdown(raw []byte) ([]CorpusPassage, error) {
	if !utf8.Valid(raw) {
		return nil, errors.New("Markdown corpus source must be valid UTF-8")
	}
	lines := strings.Split(string(raw), "\n")
	var result []CorpusPassage
	start := 0
	var buf []string
	flush := func(end int) {
		if len(buf) == 0 {
			return
		}
		text := strings.TrimSpace(strings.Join(buf, " "))
		if text != "" {
			result = append(result, CorpusPassage{Locator: fmt.Sprintf("lines:%d-%d", start+1, end), Text: text})
		}
		buf = nil
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush(i)
			continue
		}
		if len(buf) == 0 {
			start = i
		}
		buf = append(buf, strings.TrimSpace(line))
	}
	flush(len(lines))
	return result, nil
}

func extractDOCX(raw []byte) ([]CorpusPassage, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("open DOCX: %w", err)
	}
	if len(zr.File) > maxDOCXEntries {
		return nil, errors.New("DOCX contains too many ZIP entries")
	}
	var expanded uint64
	var document *zip.File
	for _, file := range zr.File {
		expanded += file.UncompressedSize64
		if expanded > maxDOCXExpandedBytes {
			return nil, errors.New("DOCX expanded content exceeds limit")
		}
		if file.Name == "word/document.xml" {
			document = file
		}
	}
	if document == nil {
		return nil, errors.New("DOCX missing word/document.xml")
	}
	if document.UncompressedSize64 > uint64(MaxCorpusSourceBytes) {
		return nil, errors.New("DOCX document XML exceeds limit")
	}
	rc, err := document.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return parseDOCXText(io.LimitReader(rc, MaxCorpusSourceBytes+1))
}

func parseDOCXText(r io.Reader) ([]CorpusPassage, error) {
	decoder := xml.NewDecoder(r)
	var result []CorpusPassage
	var paragraph strings.Builder
	inText := false
	paragraphNo := 0
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse DOCX XML: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "t" {
				inText = true
			}
		case xml.CharData:
			if inText {
				paragraph.Write([]byte(value))
			}
		case xml.EndElement:
			if value.Name.Local == "t" {
				inText = false
			}
			if value.Name.Local == "p" {
				paragraphNo++
				text := strings.TrimSpace(paragraph.String())
				paragraph.Reset()
				if text != "" {
					result = append(result, CorpusPassage{Locator: fmt.Sprintf("paragraph:%d", paragraphNo), Text: text})
				}
			}
		}
	}
	return result, nil
}

func LexicalTerms(value string) []string {
	value = strings.ToLower(value)
	value = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, value)
	set := map[string]struct{}{}
	for _, term := range strings.Fields(value) {
		if len([]rune(term)) > 1 && !retrievalStopWords[term] {
			set[term] = struct{}{}
		}
	}
	terms := make([]string, 0, len(set))
	for term := range set {
		terms = append(terms, term)
	}
	sort.Strings(terms)
	return terms
}

func RankCorpus(query string, candidates []CorpusCandidate, limit int) (RetrievalResult, error) {
	if strings.TrimSpace(query) == "" {
		return RetrievalResult{}, errors.New("query is required")
	}
	queryTerms := LexicalTerms(query)
	if len(queryTerms) == 0 {
		return RetrievalResult{Refused: true, Reason: "query contains no searchable terms"}, nil
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	querySet := make(map[string]struct{}, len(queryTerms))
	for _, term := range queryTerms {
		querySet[term] = struct{}{}
	}
	var citations []RetrievalCitation
	for _, candidate := range candidates {
		score := 0
		for _, term := range candidate.Terms {
			if _, ok := querySet[term]; ok {
				score++
			}
		}
		if score == 0 {
			continue
		}
		citations = append(citations, RetrievalCitation{
			CitationID: "corpus:" + candidate.SourceID + ":" + candidate.SHA256 + ":" + candidate.Locator,
			SourceID:   candidate.SourceID,
			Title:      candidate.Title,
			Locator:    candidate.Locator,
			Passage:    candidate.Passage,
			Score:      score,
		})
	}
	sort.SliceStable(citations, func(i, j int) bool {
		if citations[i].Score != citations[j].Score {
			return citations[i].Score > citations[j].Score
		}
		if citations[i].SourceID != citations[j].SourceID {
			return citations[i].SourceID < citations[j].SourceID
		}
		return citations[i].Locator < citations[j].Locator
	})
	if len(citations) == 0 {
		return RetrievalResult{Refused: true, Reason: "no authorised passage supports the query"}, nil
	}
	if len(citations) > limit {
		citations = citations[:limit]
	}
	return RetrievalResult{Citations: citations}, nil
}

var retrievalStopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "for": true, "how": true,
	"in": true, "is": true, "of": true, "on": true, "or": true, "the": true,
	"to": true, "what": true, "when": true,
}
