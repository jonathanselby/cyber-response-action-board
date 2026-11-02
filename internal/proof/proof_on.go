//go:build proof

package proof

import (
	"os"
	"syscall"
)

// Enabled reports whether failpoints are compiled in.
const Enabled = true

// Hit kills the process with SIGKILL when CRAB_FAILPOINT names this point, so
// the harness can prove what survives an abrupt stop at that instant.
func Hit(name string) {
	if os.Getenv("CRAB_FAILPOINT") == name {
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		select {} // never return
	}
}
