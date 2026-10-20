package pdf

import (
	"strings"
	"testing"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func TestLongReferenceWrapsWithinPrintableWidth(t *testing.T) {
	d, err := newDocument(domain.PageSizeA4, domain.PrintMonochrome, testSource(), false)
	if err != nil {
		t.Fatal(err)
	}
	width := 120.0
	lines := d.wrap("Reference: "+strings.Repeat("abcdef0123456789", 20), width)
	if len(lines) < 2 {
		t.Fatalf("long reference was not wrapped: %v", lines)
	}
	for _, line := range lines {
		measured, err := d.pdf.MeasureTextWidth(line)
		if err != nil {
			t.Fatal(err)
		}
		if measured > width+0.01 {
			t.Fatalf("line clips printable width: width=%f line=%q", measured, line)
		}
	}
}
