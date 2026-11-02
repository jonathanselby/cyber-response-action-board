//go:build !proof

package proof

import "testing"

// A normal build has no live failpoints: even with the variable set, Hit
// returns (this test would be killed otherwise).
func TestReleaseBuildHasNoFailpoints(t *testing.T) {
	t.Setenv("CRAB_FAILPOINT", "ledger-before-commit")
	if Enabled {
		t.Fatal("failpoints are compiled into a normal build")
	}
	Hit("ledger-before-commit")
}
