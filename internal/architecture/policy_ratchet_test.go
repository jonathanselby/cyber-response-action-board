package architecture

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// policyReference matches role, authorisation and Custodian checks: business
// and security policy that belongs in the application and security tiers.
var policyReference = regexp.MustCompile(`authz\.Role(Major|Minor|Observer)|authz\.Authorise\(|\.Custodian\b`)

// policyBaseline is the measured number of policy references per file in the
// presentation tier and the cmd/crab composition adapters at 66b188a, the
// start of the post-UAT remediation (§3.17 of docs/uat-remediation-triage.md).
//
// The ratchet only moves down. A file may not exceed its entry; a file not
// listed must have none; and when a file goes below its entry the entry must be
// lowered in the same change, so the improvement cannot silently regress.
// Every file touched by the remediation is expected to reach zero.
var policyBaseline = map[string]int{
	// C9 (F-7) moved the last references into app.ApprovalsService and
	// app.CapabilitiesOf. The presentation and composition tiers now hold none,
	// and none may be added.
}

// T-253: no new business logic in presentation or composition adapters.
func TestPolicyReferenceRatchet(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate source tree")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	counts := map[string]int{}
	scan := func(dir string, recursive bool) {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if !recursive && path != filepath.Join(root, dir) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if n := len(policyReference.FindAllIndex(b, -1)); n > 0 {
				rel, _ := filepath.Rel(root, path)
				counts[filepath.ToSlash(rel)] = n
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	scan("cmd/crab", false)
	scan("internal/presentation", true)

	files := map[string]bool{}
	for f := range counts {
		files[f] = true
	}
	for f := range policyBaseline {
		files[f] = true
	}
	names := make([]string, 0, len(files))
	for f := range files {
		names = append(names, f)
	}
	sort.Strings(names)
	for _, f := range names {
		got, want := counts[f], policyBaseline[f]
		switch {
		case got > want:
			t.Errorf("%s has %d policy references, baseline %d: move the rule into the application or security tier", f, got, want)
		case got < want:
			t.Errorf("%s now has %d policy references, baseline %d: lower the baseline in policyBaseline to lock the improvement", f, got, want)
		}
	}
}
