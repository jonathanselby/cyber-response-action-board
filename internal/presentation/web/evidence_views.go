package web

import (
	"mime"
	"net/http"
	"path"
	"strings"
	"unicode"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func EvidenceAttachmentHeaders(view domain.EvidenceView) http.Header {
	h := make(http.Header)
	filename := safeEvidenceFilename(view.Evidence.OriginalFilename)
	mode := domain.EvidencePresentationMode(view.Evidence.DeclaredMediaType, view.Evidence.DetectedMediaType)
	if mode == domain.EvidencePresentationInline {
		mediaType, _, err := mime.ParseMediaType(view.Evidence.DetectedMediaType)
		if err == nil {
			h.Set("Content-Type", mediaType)
		}
		h.Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": filename}))
	} else {
		h.Set("Content-Type", "application/octet-stream")
		h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	}
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-store")
	return h
}

func safeEvidenceFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		return "evidence.bin"
	}
	return name
}
