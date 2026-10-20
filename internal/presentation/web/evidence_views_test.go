package web

import (
	"strings"
	"testing"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func TestEvidenceAttachmentHeadersFailClosedAndSanitiseFilename(t *testing.T) {
	view := domain.EvidenceView{Evidence: domain.Evidence{BlobReceipt: domain.BlobReceipt{OriginalFilename: "../evil\n.html", DeclaredMediaType: "image/png", DetectedMediaType: "text/html; charset=utf-8"}}}
	h := EvidenceAttachmentHeaders(view)
	if h.Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(h.Get("Content-Disposition"), "attachment;") || strings.Contains(h.Get("Content-Disposition"), "\n") {
		t.Fatalf("unsafe evidence headers=%v", h)
	}
	if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Cache-Control") != "no-store" {
		t.Fatalf("missing hostile-byte response controls=%v", h)
	}
}
