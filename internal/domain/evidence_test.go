package domain

import "testing"

func TestEvidencePresentationFailsClosedForActiveOrMislabelledMedia(t *testing.T) {
	cases := []struct {
		declared string
		detected string
		want     string
	}{
		{"image/png", "text/html; charset=utf-8", EvidencePresentationDownload},
		{"text/html", "text/html; charset=utf-8", EvidencePresentationDownload},
		{"image/svg+xml", "image/svg+xml", EvidencePresentationDownload},
		{"application/pdf", "application/pdf", EvidencePresentationDownload},
		{"image/jpeg", "image/png", EvidencePresentationDownload},
		{"image/png", "image/png", EvidencePresentationInline},
		{"", "image/jpeg", EvidencePresentationInline},
	}
	for _, tc := range cases {
		if got := EvidencePresentationMode(tc.declared, tc.detected); got != tc.want {
			t.Fatalf("declared=%q detected=%q got=%q want=%q", tc.declared, tc.detected, got, tc.want)
		}
	}
}
