//go:build proof

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Load and soak proofs (C11; R-10, R-11, F-5; T-238, T-239).
//
// CRAB_PROOF_PROFILE=full runs the long profile; the default "short" profile
// is what CI and the Pi command run. CRAB_PROOF_REPORT_DIR, if set, receives a
// Markdown report for the build evidence.

type profile struct {
	name             string
	users            int
	loadFor, soakFor time.Duration
	soakSimulated    time.Duration
	think            time.Duration
}

func proofProfile() profile {
	if os.Getenv("CRAB_PROOF_PROFILE") == "full" {
		return profile{name: "full", users: 50, loadFor: 10 * time.Minute, soakFor: 6 * time.Hour, soakSimulated: 72 * time.Hour, think: 100 * time.Millisecond}
	}
	return profile{name: "short", users: 50, loadFor: 60 * time.Second, soakFor: 3 * time.Minute, soakSimulated: 72 * time.Hour, think: 100 * time.Millisecond}
}

// loadFixture seeds the volumes the load runs against: the board stress
// incident, five working incidents, and n responders, each a member of one
// working incident. Each incident carries a scope marker that must never
// appear to people outside it.
func (e *e2e) loadFixture(n int) (incidentOf map[string]string) {
	e.t.Helper()
	alice, bob := e.bootstrap()
	incidentOf = map[string]string{}
	// Board-stress volumes on every working incident, through the ledger only
	// so the oracle's rebuild check holds (the visual stress fixture also
	// writes projection-only rows).
	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("INC-LD%d", i)
		e.declare(alice, bob, id, fmt.Sprintf("Load incident %d", i), "Severity level 3")
		e.setMember(alice, bob, id, "bob", true)
		e.post("/incident/"+id+"/fact", alice, url.Values{"statement": {"SCOPE-MARKER-" + id}, "source": {"fixture"}})
		for j := 0; j < 41; j++ {
			e.post("/incident/"+id+"/action", alice, url.Values{"title": {fmt.Sprintf("Load action %02d", j)}, "owner": {"bob"}, "priority": {fmt.Sprintf("P%d", 1+j%3)}, "source_ref": {"fixture"}})
		}
		for j := 0; j < 18; j++ {
			e.post("/incident/"+id+"/decision", alice, url.Values{"decision": {fmt.Sprintf("Load decision %02d", j)}, "rationale": {"fixture"}})
		}
		e.approveAll(bob)
		for j := 0; j < 13; j++ {
			e.post("/incident/"+id+"/uncertainty", alice, url.Values{"kind": {"unknown"}, "statement": {fmt.Sprintf("Load unknown %02d", j)}})
		}
		for j := 0; j < 8; j++ {
			e.post("/incident/"+id+"/decision", alice, url.Values{"decision": {fmt.Sprintf("Waiting decision %d", j)}, "rationale": {"fixture"}})
		}
	}
	for u := 1; u <= n; u++ {
		id := fmt.Sprintf("u%02d", u)
		e.onboard(alice, bob, id, "Minor", false)
		inc := fmt.Sprintf("INC-LD%d", 1+u%5)
		e.setMember(alice, bob, inc, id, true)
		incidentOf[id] = inc
	}
	return incidentOf
}

type opStats struct {
	mu        sync.Mutex
	latencies map[string][]time.Duration
	status    map[int]int
	unusual   map[string]int // "op status" for anything but 200 and 303
	writes    []string       // statements of facts the server accepted (303)
	leaks     []string
}

func newOpStats() *opStats {
	return &opStats{latencies: map[string][]time.Duration{}, status: map[int]int{}, unusual: map[string]int{}}
}

func (s *opStats) add(op string, d time.Duration, code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latencies[op] = append(s.latencies[op], d)
	s.status[code]++
	if code != http.StatusOK && code != http.StatusSeeOther {
		s.unusual[fmt.Sprintf("%s %d", op, code)]++
	}
}

func percentile(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), ds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[int(float64(len(sorted)-1)*p)]
}

// drive runs one responder until the deadline: Command Summary, their Board,
// Approvals, Evidence, and a fact every third round.
func drive(t *testing.T, base, id, incident string, deadline time.Time, think time.Duration, stats *opStats, others []string) {
	c := signInTo(t, base, id, "Onboard-"+id+"-26!")
	timed := func(op, path string) string {
		start := time.Now()
		res, err := c.http.Get(base + path)
		if err != nil {
			stats.add(op, time.Since(start), 0)
			return ""
		}
		buf := new(strings.Builder)
		_, _ = fmt.Fprint(buf, readAll(res))
		stats.add(op, time.Since(start), res.StatusCode)
		if res.StatusCode == http.StatusSeeOther || strings.Contains(buf.String(), `action="/login"`) {
			c = signInTo(t, base, id, "Onboard-"+id+"-26!") // session ended; sign in again
		}
		return buf.String()
	}
	for round := 0; time.Now().Before(deadline); round++ {
		pages := []string{timed("command-summary", "/"), timed("board", "/incident/"+incident), timed("approvals", "/approvals"), timed("evidence", "/incident/"+incident+"/evidence")}
		for _, page := range pages {
			for _, other := range others {
				if other != incident && strings.Contains(page, "SCOPE-MARKER-"+other) {
					stats.mu.Lock()
					stats.leaks = append(stats.leaks, id+" saw "+other)
					stats.mu.Unlock()
				}
			}
		}
		if round%3 == 0 {
			statement := fmt.Sprintf("load %s round %d", id, round)
			start := time.Now()
			res, err := c.form("/incident/"+incident+"/fact", url.Values{"statement": {statement}, "source": {"load"}})
			code := 0
			if err == nil {
				code = res.StatusCode
			}
			stats.add("record-fact", time.Since(start), code)
			if code == http.StatusSeeOther {
				stats.mu.Lock()
				stats.writes = append(stats.writes, statement)
				stats.mu.Unlock()
			}
		}
		time.Sleep(think)
	}
}

func readAll(res *http.Response) string {
	defer res.Body.Close()
	b := new(strings.Builder)
	buf := make([]byte, 32<<10)
	for {
		n, err := res.Body.Read(buf)
		b.Write(buf[:n])
		if err != nil {
			return b.String()
		}
	}
}

type procSample struct {
	rssKB, cpuTicks, conns int
}

func sampleProcess(e *e2e, pid int) procSample {
	var s procSample
	if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "VmRSS:") {
				s.rssKB, _ = strconv.Atoi(strings.Fields(line)[1])
			}
		}
	}
	if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
		f := strings.Fields(string(b))
		if len(f) > 15 {
			u, _ := strconv.Atoi(f[13])
			k, _ := strconv.Atoi(f[14])
			s.cpuTicks = u + k
		}
	}
	_ = e.admin.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND usename=$1`, e2eRuntimeRole).Scan(&s.conns)
	return s
}

type serverStat struct {
	Goroutines int    `json:"goroutines"`
	HeapAlloc  uint64 `json:"heap_alloc"`
}

func readServerStats(t *testing.T, path string) []serverStat {
	b, _ := os.ReadFile(path)
	var out []serverStat
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var s serverStat
		if json.Unmarshal([]byte(line), &s) == nil {
			out = append(out, s)
		}
	}
	return out
}

func report(t *testing.T, name, body string) {
	t.Log("\n" + body)
	if dir := os.Getenv("CRAB_PROOF_REPORT_DIR"); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
	}
}

// T-238 (R-10, F-5): 50 responders, each with their own principal and scope,
// against the board stress fixture and five working incidents, through the
// real `crab serve`. The oracle passes; no 5xx except declared 503
// back-pressure; nobody sees another incident; latency and resources are
// recorded.
func TestProofLoad(t *testing.T) {
	prof := proofProfile()
	e := newE2E(t)
	seedStart := time.Now()
	incidentOf := e.loadFixture(prof.users)
	seeded := time.Since(seedStart)
	stats := filepath.Join(t.TempDir(), "stats.jsonl")
	t.Setenv("CRAB_PROOF_STATS", stats)
	if dir := os.Getenv("CRAB_PROOF_REPORT_DIR"); dir != "" {
		t.Setenv("CRAB_PROOF_CPUPROFILE", filepath.Join(dir, "load-cpu.pprof"))
	}
	srv := startCrab(t, proofBinary(t), "")
	pid := srv.cmd.Process.Pid
	others := []string{"INC-LD1", "INC-LD2", "INC-LD3", "INC-LD4", "INC-LD5"}
	ops := newOpStats()
	deadline := time.Now().Add(prof.loadFor)
	var wg sync.WaitGroup
	for id, inc := range incidentOf {
		wg.Add(1)
		go func(id, inc string) {
			defer wg.Done()
			drive(t, srv.base, id, inc, deadline, prof.think, ops, others)
		}(id, inc)
	}
	start := sampleProcess(e, pid)
	var samples []procSample
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	for waiting := true; waiting; {
		select {
		case <-done:
			waiting = false
		case <-time.After(time.Second):
			samples = append(samples, sampleProcess(e, pid))
		}
	}
	end := sampleProcess(e, pid)
	srv.stop()

	maxRSS, maxConns := 0, 0
	for _, s := range samples {
		if s.rssKB > maxRSS {
			maxRSS = s.rssKB
		}
		if s.conns > maxConns {
			maxConns = s.conns
		}
	}
	server := readServerStats(t, stats)
	maxG, maxHeap := 0, uint64(0)
	for _, s := range server {
		if s.Goroutines > maxG {
			maxG = s.Goroutines
		}
		if s.HeapAlloc > maxHeap {
			maxHeap = s.HeapAlloc
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Load (T-238), profile %s\n\n%d responders for %s; fixture seeded in %s; GOMAXPROCS %d on %s/%s.\n\n", prof.name, prof.users, prof.loadFor, seeded.Round(time.Second), runtime.GOMAXPROCS(0), runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "| Operation | Requests | p50 | p95 | p99 |\n|---|---|---|---|---|\n")
	total := 0
	names := make([]string, 0, len(ops.latencies))
	for op := range ops.latencies {
		names = append(names, op)
	}
	sort.Strings(names)
	for _, op := range names {
		ds := ops.latencies[op]
		total += len(ds)
		fmt.Fprintf(&b, "| %s | %d | %s | %s | %s |\n", op, len(ds), percentile(ds, .5).Round(time.Millisecond), percentile(ds, .95).Round(time.Millisecond), percentile(ds, .99).Round(time.Millisecond))
	}
	fmt.Fprintf(&b, "\n%d requests (%.0f per second). Status counts: %v. Other than 200 and 303, by operation: %v.\n\n", total, float64(total)/prof.loadFor.Seconds(), ops.status, ops.unusual)
	fmt.Fprintf(&b, "Server process: peak RSS %d MiB; CPU %.1f s over the run; peak goroutines %d; peak heap %d MiB; peak PostgreSQL connections %d (pool cap 8).\n", maxRSS/1024, float64(end.cpuTicks-start.cpuTicks)/100, maxG, maxHeap>>20, maxConns)
	report(t, "load.md", b.String())

	for code, n := range ops.status {
		if code >= 500 && code != http.StatusServiceUnavailable {
			t.Errorf("%d responses with status %d", n, code)
		}
		if code == 0 {
			t.Errorf("%d requests got no response", n)
		}
	}
	if len(ops.leaks) > 0 {
		t.Fatalf("scope leaks: %v", ops.leaks[:min(5, len(ops.leaks))])
	}
	// Every accepted write is exactly one record; none is missing or doubled.
	var recorded int
	if err := e.admin.QueryRow(context.Background(), `SELECT count(*) FROM crab.incident_facts WHERE source='load'`).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != len(ops.writes) {
		t.Fatalf("%d accepted writes but %d load facts recorded", len(ops.writes), recorded)
	}
	var distinct int
	_ = e.admin.QueryRow(context.Background(), `SELECT count(DISTINCT statement) FROM crab.incident_facts WHERE source='load'`).Scan(&distinct)
	if distinct != recorded {
		t.Fatalf("duplicate load facts: %d distinct of %d", distinct, recorded)
	}
	e.oracle("after load")
}

// T-239 (R-11, F-3): an accelerated 72-hour soak in-process on the governed
// clock: responders keep working while the clock races through three days,
// signing in again whenever their eight-hour session ends. After warm-up,
// heap, goroutines, sessions and PostgreSQL connections must not keep
// growing, and the oracle passes.
func TestProofSoak(t *testing.T) {
	prof := proofProfile()
	clk := &testClock{now: time.Now().UTC()}
	e := newE2EWithClock(t, clk)
	const users = 10
	incidentOf := e.loadFixture(users)
	server := httptest.NewServer(e.handler)
	defer server.Close()
	others := []string{"INC-LD1", "INC-LD2", "INC-LD3", "INC-LD4", "INC-LD5"}
	ops := newOpStats()
	deadline := time.Now().Add(prof.soakFor)
	step := time.Duration(float64(prof.soakSimulated) / prof.soakFor.Seconds()) // simulated time per real second
	var wg sync.WaitGroup
	for id, inc := range incidentOf {
		wg.Add(1)
		go func(id, inc string) {
			defer wg.Done()
			drive(t, server.URL, id, inc, deadline, 250*time.Millisecond, ops, others)
		}(id, inc)
	}
	type soakSample struct {
		heap                        uint64
		goroutines, sessions, conns int
	}
	var samples []soakSample
	every := prof.soakFor / 30
	next := time.Now().Add(every)
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)
		clk.Advance(step)
		if time.Now().After(next) {
			next = next.Add(every)
			runtime.GC()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			s := soakSample{heap: m.HeapAlloc, goroutines: runtime.NumGoroutine()}
			_ = e.admin.QueryRow(context.Background(), `SELECT count(*) FROM crab.sessions`).Scan(&s.sessions)
			_ = e.admin.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND usename=$1`, e2eRuntimeRole).Scan(&s.conns)
			samples = append(samples, s)
		}
	}
	wg.Wait()
	if len(samples) < 9 {
		t.Fatalf("only %d samples", len(samples))
	}
	third := len(samples) / 3
	middle, last := samples[third:2*third], samples[2*third:]
	peak := func(ss []soakSample, f func(soakSample) float64) float64 {
		m := 0.0
		for _, s := range ss {
			if v := f(s); v > m {
				m = v
			}
		}
		return m
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Soak (T-239), profile %s\n\n%d responders for %s real time; the governed clock advanced %s (%s per real second), so sessions expired and were renewed about %d times each.\n\n", prof.name, users, prof.soakFor, prof.soakSimulated, step.Round(time.Second), int(prof.soakSimulated/(8*time.Hour)))
	fmt.Fprintf(&b, "| Sample | Heap MiB | Goroutines | Sessions | PostgreSQL connections |\n|---|---|---|---|---|\n")
	for i, s := range samples {
		fmt.Fprintf(&b, "| %d | %.1f | %d | %d | %d |\n", i+1, float64(s.heap)/(1<<20), s.goroutines, s.sessions, s.conns)
	}
	fmt.Fprintf(&b, "\nRequests: %d status counts %v. Accepted writes: %d.\n", func() int {
		n := 0
		for _, v := range ops.status {
			n += v
		}
		return n
	}(), ops.status, len(ops.writes))
	fmt.Fprintf(&b, "Responses other than 200 and 303, by operation: %v.\n", ops.unusual)
	report(t, "soak.md", b.String())

	checks := []struct {
		name  string
		f     func(soakSample) float64
		slack float64
		scale float64
	}{
		{"heap", func(s soakSample) float64 { return float64(s.heap) }, 8 << 20, 1.5},
		{"goroutines", func(s soakSample) float64 { return float64(s.goroutines) }, 20, 1.2},
		{"sessions", func(s soakSample) float64 { return float64(s.sessions) }, 30, 1.5},
		{"connections", func(s soakSample) float64 { return float64(s.conns) }, 2, 1.0},
	}
	for _, c := range checks {
		if late, mid := peak(last, c.f), peak(middle, c.f); late > mid*c.scale+c.slack {
			t.Errorf("%s keeps growing after warm-up: middle peak %.0f, last peak %.0f", c.name, mid, late)
		}
	}
	for code, n := range ops.status {
		if code >= 500 && code != http.StatusServiceUnavailable {
			t.Errorf("%d responses with status %d", n, code)
		}
	}
	if len(ops.leaks) > 0 {
		t.Fatalf("scope leaks: %v", ops.leaks[:min(5, len(ops.leaks))])
	}
	var recorded int
	_ = e.admin.QueryRow(context.Background(), `SELECT count(*) FROM crab.incident_facts WHERE source='load'`).Scan(&recorded)
	if recorded != len(ops.writes) {
		t.Fatalf("%d accepted writes but %d load facts recorded", len(ops.writes), recorded)
	}
	e.oracle("after soak")
}
