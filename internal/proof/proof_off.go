//go:build !proof

// Package proof holds test-only failpoints for the destructive and upgrade
// harnesses (R-12, R-15). In a normal build every failpoint is an empty
// function, so release binaries contain no way to trigger one.
package proof

// Enabled reports whether failpoints are compiled in.
const Enabled = false

// Hit is a named failpoint. It does nothing in a release build.
func Hit(string) {}
