package domain

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestCorpusExtractionMarkdownAndDOCXIsDeterministicAndPassive(t *testing.T) {
	markdown := []byte("# Isolation\nDisconnect affected hosts from the network.\n\n# Recovery\nRestore from known-good backups.\n")
	first, err := ExtractCorpus(CorpusFormatMarkdown, markdown)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExtractCorpus(CorpusFormatMarkdown, markdown)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(second) != 2 || first[0].Locator != "lines:1-2" || first[0].Locator != second[0].Locator || strings.Join(first[0].Terms, ",") != strings.Join(second[0].Terms, ",") {
		t.Fatalf("unstable Markdown extraction first=%+v second=%+v", first, second)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	doc, _ := zw.Create("word/document.xml")
	_, _ = doc.Write([]byte(`<?xml version="1.0"?><w:document xmlns:w="urn:w"><w:body><w:p><w:r><w:t>Call the incident commander.</w:t></w:r></w:p><w:p><w:r><w:t>Preserve evidence.</w:t></w:r></w:p></w:body></w:document>`))
	macro, _ := zw.Create("word/vbaProject.bin")
	_, _ = macro.Write([]byte("hostile macro bytes must never execute or enter retrieval"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	passages, err := ExtractCorpus(CorpusFormatDOCX, buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(passages) != 2 || passages[0].Locator != "paragraph:1" || strings.Contains(strings.Join([]string{passages[0].Text, passages[1].Text}, " "), "macro") {
		t.Fatalf("DOCX extraction included non-document active content: %+v", passages)
	}
}

func TestCorpusDOCXEntryLimitAndRetrievalRefusal(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	doc, _ := zw.Create("word/document.xml")
	_, _ = doc.Write([]byte(`<w:document xmlns:w="urn:w"><w:body><w:p><w:r><w:t>safe text</w:t></w:r></w:p></w:body></w:document>`))
	for i := 0; i < maxDOCXEntries; i++ {
		entry, _ := zw.Create("junk/" + strings.Repeat("x", 8) + string(rune('A'+i%26)) + strings.Repeat("y", i%7))
		_, _ = entry.Write([]byte("x"))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractCorpus(CorpusFormatDOCX, buf.Bytes()); err == nil || !strings.Contains(err.Error(), "too many ZIP entries") {
		t.Fatalf("entry limit err=%v", err)
	}

	result, err := RankCorpus("unrelated satellite telemetry", []CorpusCandidate{{SourceID: "RUNBOOK", Title: "Runbook", SHA256: strings.Repeat("a", 64), Locator: "lines:1-2", Passage: "Disconnect ransomware hosts", Terms: LexicalTerms("Disconnect ransomware hosts")}}, 5)
	if err != nil || !result.Refused || len(result.Citations) != 0 {
		t.Fatalf("unsupported retrieval result=%+v err=%v", result, err)
	}
}

func TestCorpusRankingStableCitationAndTieBreak(t *testing.T) {
	candidates := []CorpusCandidate{
		{SourceID: "B", Title: "B", SHA256: strings.Repeat("b", 64), Locator: "lines:1-1", Passage: "isolate host", Terms: LexicalTerms("isolate host")},
		{SourceID: "A", Title: "A", SHA256: strings.Repeat("a", 64), Locator: "lines:2-2", Passage: "isolate host", Terms: LexicalTerms("isolate host")},
	}
	first, err := RankCorpus("isolate host", candidates, 5)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RankCorpus("isolate host", candidates, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Citations) != 2 || first.Citations[0].SourceID != "A" || first.Citations[0].CitationID != second.Citations[0].CitationID {
		t.Fatalf("unstable ranking first=%+v second=%+v", first, second)
	}
}
