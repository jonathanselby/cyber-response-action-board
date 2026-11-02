//go:build proof

package proof

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"
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

// StartSampler writes goroutine and heap figures to the file named by
// CRAB_PROOF_STATS every second, one JSON line each, so the load harness can
// see inside the server process.
func StartSampler() {
	if prof := os.Getenv("CRAB_PROOF_CPUPROFILE"); prof != "" {
		if f, err := os.Create(prof); err == nil {
			_ = pprof.StartCPUProfile(f)
		}
	}
	path := os.Getenv("CRAB_PROOF_STATS")
	if path == "" {
		return
	}
	go func() {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return
		}
		defer f.Close()
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			fmt.Fprintf(f, "{\"at\":%d,\"goroutines\":%d,\"heap_alloc\":%d,\"sys\":%d,\"num_gc\":%d}\n", time.Now().UnixMilli(), runtime.NumGoroutine(), m.HeapAlloc, m.Sys, m.NumGC)
			time.Sleep(time.Second)
		}
	}()
}

// Stop flushes any CPU profile; called when the server shuts down.
func Stop() { pprof.StopCPUProfile() }
