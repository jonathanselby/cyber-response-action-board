package web

import (
	"errors"
	"strings"
	"testing"
)

// T-241 follow-up: a disk-full or other database refusal is shown in plain
// words, never as PostgreSQL's error text.
func TestDatabaseRefusalsAreWordedForPeople(t *testing.T) {
	full := errors.New(`commit ledger append: ERROR: could not extend file "pg_tblspc/16384/PG_16/16385/16400": No space left on device (SQLSTATE 53100)`)
	got := refusalText(full)
	if !strings.HasPrefix(got, "Nothing was saved") || strings.Contains(got, "SQLSTATE") || strings.Contains(got, "pg_tblspc") {
		t.Fatalf("disk full shown as %q", got)
	}
	other := errors.New(`insert ledger entry: ERROR: duplicate key value violates unique constraint "x" (SQLSTATE 23505)`)
	if got := refusalText(other); !strings.HasPrefix(got, "Nothing was saved") || strings.Contains(got, "duplicate key") {
		t.Fatalf("database refusal shown as %q", got)
	}
	if got := refusalText(errors.New("address required")); got != "Address required" {
		t.Fatalf("ordinary refusal changed: %q", got)
	}
}
