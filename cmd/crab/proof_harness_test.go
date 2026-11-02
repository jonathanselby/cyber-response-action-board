//go:build proof

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
)

// Proof harness (C10; triage §6). Built only with -tags proof, which also
// compiles the failpoints in internal/proof. It runs the real `crab` binary
// as a subprocess against PostgreSQL and checks one rule after every
// disruption: CRAB may become slower; it must not become wrong.

// proofBinary builds crab with failpoints once per test.
func proofBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "crab-proof")
	cmd := exec.Command("go", "build", "-tags", "proof", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build proof binary: %v\n%s", err, out)
	}
	return bin
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

// crabServer is one `crab serve` subprocess.
type crabServer struct {
	t    *testing.T
	cmd  *exec.Cmd
	base string
	log  *bytes.Buffer
}

func startCrab(t *testing.T, bin, failpoint string) *crabServer {
	t.Helper()
	addr := freeAddr(t)
	cmd := exec.Command(bin, "serve")
	cmd.Env = append(os.Environ(), "CRAB_LISTEN_ADDR="+addr, "CRAB_FAILPOINT="+failpoint)
	log := &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s := &crabServer{t: t, cmd: cmd, base: "http://" + addr, log: log}
	deadline := time.Now().Add(20 * time.Second)
	for {
		if res, err := http.Get(s.base + "/healthz"); err == nil {
			res.Body.Close()
			return s
		}
		if time.Now().After(deadline) {
			t.Fatalf("crab serve did not start:\n%s", log)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// waitKilled waits for the process to die and reports whether SIGKILL did it.
func (s *crabServer) waitKilled() bool {
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		_ = s.cmd.Process.Kill()
		<-done
		return false
	}
	if status, ok := s.cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
		return status.Signaled() && status.Signal() == syscall.SIGKILL
	}
	return false
}

func (s *crabServer) stop() {
	_ = s.cmd.Process.Signal(syscall.SIGTERM)
	_, _ = s.cmd.Process.Wait()
}

// client is a signed-in browser.
type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func signInTo(t *testing.T, base, id, password string) *client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &client{t: t, base: base, http: &http.Client{Jar: jar, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	res, err := c.form("/login", url.Values{"human_id": {id}, "password": {password}})
	if err != nil || res.StatusCode != http.StatusSeeOther {
		t.Fatalf("sign in to %s: %v %v", base, err, res)
	}
	return c
}

func (c *client) form(path string, v url.Values) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodPost, c.base+path, strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", c.base)
	res, err := c.http.Do(req)
	if err == nil {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
	}
	return res, err
}

func (c *client) upload(incidentID, description string) (*http.Response, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range map[string]string{"compartment": "GENERAL", "description": description, "collector": "alice", "source": "laptop", "current_handler": "alice"} {
		_ = w.WriteField(k, v)
	}
	f, _ := w.CreateFormFile("evidence", "memory.raw")
	_, _ = f.Write(bytes.Repeat([]byte(description), 4096))
	_ = w.Close()
	req, _ := http.NewRequest(http.MethodPost, c.base+"/incident/"+incidentID+"/evidence?compartment=GENERAL", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Origin", c.base)
	res, err := c.http.Do(req)
	if err == nil {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
	}
	return res, err
}

// oracle checks the correctness rule: the ledger verifies, projections
// rebuild byte-identically, every blob re-hashes to its recorded SHA-256.
func (e *e2e) oracle(step string) {
	e.t.Helper()
	ctx := context.Background()
	keys, err := loadKeyring(os.Getenv("CRAB_LEDGER_KEYRING_FILE"))
	if err != nil {
		e.t.Fatal(err)
	}
	pool, err := postgres.OpenPool(ctx, os.Getenv("CRAB_DATABASE_URL"))
	if err != nil {
		e.t.Fatal(err)
	}
	defer pool.Close()
	kernel, err := postgres.OpenKernel(ctx, pool, keys)
	if err != nil {
		e.t.Fatalf("%s: open kernel: %v", step, err)
	}
	if res, err := kernel.Verify(ctx); err != nil || !res.Valid {
		e.t.Fatalf("%s: ledger does not verify: %v %+v", step, err, res)
	}
	before := e.projectionSnapshot()
	if _, err := kernel.Rebuild(ctx, postgres.RebuildProjections()...); err != nil {
		e.t.Fatalf("%s: rebuild: %v", step, err)
	}
	if after := e.projectionSnapshot(); after != before {
		b, a := strings.Split(before, "\n"), strings.Split(after, "\n")
		var diff []string
		for i := range b {
			if i < len(a) && a[i] != b[i] {
				diff = append(diff, strings.SplitN(b[i], "=", 2)[0])
			}
		}
		e.t.Fatalf("%s: projections did not rebuild identically: %s", step, strings.Join(diff, ", "))
	}
	blobs, err := postgres.NewBlobStore(pool, 64<<20)
	if err != nil {
		e.t.Fatal(err)
	}
	rows, err := e.admin.Query(ctx, `SELECT blob_id FROM crab.blobs`)
	if err != nil {
		e.t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if _, err := blobs.Verify(ctx, id); err != nil {
			e.t.Fatalf("%s: blob %s does not re-hash: %v", step, id, err)
		}
	}
}

func (e *e2e) countWhere(sql string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.admin.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// T-240 (R-12, F-4): at every kill point the operation is committed once or
// not at all; the oracle passes after restart; orphan blobs are reported.
func TestProofDestructive(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-K", "Kill points", "Severity level 3")
	bin := proofBinary(t)
	evidence := func(marker string) int {
		return e.countWhere(`SELECT count(*) FROM crab.evidence_register WHERE description=$1`, marker)
	}
	for _, kp := range []struct {
		point     string
		committed int // evidence records after restart
		orphan    int // new stored blobs with no record
	}{
		{"blob-stored", 0, 1},
		{"ledger-before-commit", 0, 1},
		{"ledger-after-commit", 1, 0},
	} {
		orphansBefore := len(e.orphans())
		ledgerBefore := e.ledgerCount()
		srv := startCrab(t, bin, kp.point)
		c := signInTo(t, srv.base, "alice", e2eAlicePassword)
		marker := "kill at " + kp.point
		if res, err := c.upload("INC-K", marker); err == nil {
			t.Fatalf("%s: the request succeeded (%d) although the process was killed", kp.point, res.StatusCode)
		}
		if !srv.waitKilled() {
			t.Fatalf("%s: process was not killed at the failpoint:\n%s", kp.point, srv.log)
		}
		srv = startCrab(t, bin, "")
		if got := evidence(marker); got != kp.committed {
			t.Errorf("%s: %d evidence records after restart, want %d", kp.point, got, kp.committed)
		}
		if got := e.ledgerCount() - ledgerBefore; got != kp.committed {
			t.Errorf("%s: %d ledger records written, want %d", kp.point, got, kp.committed)
		}
		if got := len(e.orphans()) - orphansBefore; got != kp.orphan {
			t.Errorf("%s: %d new orphan blobs, want %d", kp.point, got, kp.orphan)
		}
		// The restarted server works, and a 2xx mutation is exactly one record.
		c = signInTo(t, srv.base, "alice", e2eAlicePassword)
		fact := "after restart from " + kp.point
		if res, err := c.form("/incident/INC-K/fact", url.Values{"statement": {fact}, "source": {"proof"}}); err != nil || res.StatusCode != http.StatusSeeOther {
			t.Fatalf("%s: restarted server refused a fact: %v %v", kp.point, err, res)
		}
		if n := e.countWhere(`SELECT count(*) FROM crab.incident_facts WHERE statement=$1`, fact); n != 1 {
			t.Fatalf("%s: a 2xx fact produced %d records", kp.point, n)
		}
		srv.stop()
		e.oracle(kp.point)
	}
	// Orphans are reported by readiness, not hidden or deleted.
	out, _ := exec.Command(bin, "readiness").CombinedOutput()
	if !strings.Contains(string(out), `"orphan_blobs"`) || !strings.Contains(string(out), "2 stored file(s) have no record") {
		t.Fatalf("readiness does not report the orphan blobs:\n%s", out)
	}
	if n := e.countWhere(`SELECT count(*) FROM crab.blobs`); n < 3 {
		t.Fatalf("orphan blobs were deleted (%d blobs left)", n)
	}

	// A backup killed part-way leaves no backup at the output path, and the
	// next backup succeeds and verifies.
	dir := t.TempDir()
	out1 := filepath.Join(dir, "killed.dump")
	cmd := exec.Command(bin, "backup", "--output", out1)
	cmd.Env = append(os.Environ(), "CRAB_FAILPOINT=backup-mid")
	_ = cmd.Run()
	if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); !ok || !status.Signaled() {
		t.Fatalf("backup was not killed at backup-mid")
	}
	if _, err := os.Stat(out1); err == nil {
		t.Fatal("a killed backup left a file at the output path")
	}
	out2 := filepath.Join(dir, "good.dump")
	if b, err := exec.Command(bin, "backup", "--output", out2).CombinedOutput(); err != nil {
		t.Fatalf("backup after a killed backup failed: %v\n%s", err, b)
	}
	if err := postgres.VerifyBackup(out2); err != nil {
		t.Fatalf("backup after a killed backup does not verify: %v", err)
	}
	e.oracle("after backups")
}

func (e *e2e) orphans() []string {
	e.t.Helper()
	pool, err := postgres.OpenPool(context.Background(), os.Getenv("CRAB_DATABASE_URL"))
	if err != nil {
		e.t.Fatal(err)
	}
	defer pool.Close()
	ids, err := postgres.OrphanBlobs(context.Background(), pool)
	if err != nil {
		e.t.Fatal(err)
	}
	return ids
}

func shell(t *testing.T, command string) {
	t.Helper()
	if out, err := exec.Command("sh", "-c", command).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", command, err, out)
	}
}

// postFact records a fact and returns the status, or 0 if no response came.
func (c *client) postFact(incidentID, statement string) int {
	res, err := c.form("/incident/"+incidentID+"/fact", url.Values{"statement": {statement}, "source": {"proof"}})
	if err != nil {
		return 0
	}
	return res.StatusCode
}

// T-241 (R-13): with PostgreSQL down or its disk full, no mutation reports
// success without a commit, the failure is visible, and service resumes when
// the dependency returns, without restarting CRAB.
//
// Needs CRAB_PROOF_PG_STOP and CRAB_PROOF_PG_START (shell commands) and
// CRAB_PROOF_FULL_DIR (a small filesystem the PostgreSQL server can write).
func TestProofDatabaseAndStorage(t *testing.T) {
	stop, start, full := os.Getenv("CRAB_PROOF_PG_STOP"), os.Getenv("CRAB_PROOF_PG_START"), os.Getenv("CRAB_PROOF_FULL_DIR")
	if stop == "" || start == "" || full == "" {
		t.Skip("CRAB_PROOF_PG_STOP, CRAB_PROOF_PG_START and CRAB_PROOF_FULL_DIR not set")
	}
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-DB", "Database outage", "Severity level 3")
	bin := proofBinary(t)
	srv := startCrab(t, bin, "")
	defer srv.stop()
	c := signInTo(t, srv.base, "alice", e2eAlicePassword)
	facts := func(s string) int {
		return e.countWhere(`SELECT count(*) FROM crab.incident_facts WHERE statement=$1`, s)
	}
	retry := func(statement string) {
		t.Helper()
		for i := 0; i < 60; i++ {
			if c.postFact("INC-DB", statement) == http.StatusSeeOther {
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
		res, err := c.form("/incident/INC-DB/fact", url.Values{"statement": {statement}, "source": {"proof"}})
		detail := fmt.Sprint(err)
		if err == nil {
			detail = fmt.Sprintf("status %d", res.StatusCode)
		}
		t.Fatalf("service did not resume for %q (%s):\n%s", statement, detail, srv.log)
	}

	// PostgreSQL stopped.
	shell(t, stop)
	if code := c.postFact("INC-DB", "written while PostgreSQL was down"); code == http.StatusSeeOther {
		shell(t, start)
		t.Fatal("a mutation reported success while PostgreSQL was down")
	}
	res, err := c.http.Get(srv.base + "/")
	shell(t, start)
	if err == nil {
		res.Body.Close()
		if res.StatusCode == http.StatusOK {
			t.Error("the Command Summary rendered as normal while PostgreSQL was down")
		}
	}
	retry("written after PostgreSQL returned")
	if facts("written while PostgreSQL was down") != 0 || facts("written after PostgreSQL returned") != 1 {
		t.Fatal("outage left a wrong record count")
	}

	// PostgreSQL's disk full: the ledger and facts live on a small tablespace.
	ctx := context.Background()
	exec := func(sql string) {
		t.Helper()
		if _, err := e.admin.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec(`CREATE TABLESPACE crab_proof_full LOCATION '` + full + `'`)
	defer func() {
		_, _ = e.admin.Exec(ctx, `DROP TABLE IF EXISTS crab_proof_filler`)
		_, _ = e.admin.Exec(ctx, `ALTER TABLE crab.ledger_entries SET TABLESPACE pg_default`)
		_, _ = e.admin.Exec(ctx, `ALTER TABLE crab.incident_facts SET TABLESPACE pg_default`)
		_, _ = e.admin.Exec(ctx, `DROP TABLESPACE IF EXISTS crab_proof_full`)
	}()
	exec(`ALTER TABLE crab.ledger_entries SET TABLESPACE crab_proof_full`)
	exec(`ALTER TABLE crab.incident_facts SET TABLESPACE crab_proof_full`)
	exec(`CREATE TABLE crab_proof_filler (pad text) TABLESPACE crab_proof_full`)
	// Fill in shrinking steps so no page keeps room for a small row.
	filled := false
	for _, rowChars := range []int{64000, 4000, 256, 16} {
		for i := 0; i < 20000; i++ {
			_, err := e.admin.Exec(ctx, `INSERT INTO crab_proof_filler SELECT repeat(md5(random()::text), $1)`, rowChars/32)
			if err == nil {
				continue
			}
			if !strings.Contains(err.Error(), "No space left") && !strings.Contains(err.Error(), "could not extend") {
				t.Fatalf("filling the tablespace: %v", err)
			}
			filled = true
			break
		}
	}
	if !filled {
		t.Fatal("could not fill the tablespace")
	}
	// Writes may still fit in partly filled pages; every success must be a
	// commit, and once space runs out the refusal must be visible and leave
	// nothing behind.
	refused := false
	for i := 0; i < 500 && !refused; i++ {
		statement := fmt.Sprintf("written while the disk filled %d", i)
		ledger := e.ledgerCount()
		code := c.postFact("INC-DB", statement)
		switch {
		case code == 0:
			t.Fatalf("CRAB stopped responding with the disk full:\n%s", srv.log)
		case code == http.StatusSeeOther:
			if e.ledgerCount() != ledger+1 || facts(statement) != 1 {
				t.Fatalf("write %d reported success without exactly one commit", i)
			}
		default:
			refused = true
			if e.ledgerCount() != ledger || facts(statement) != 0 {
				t.Fatalf("a refused write left a record")
			}
		}
	}
	if !refused {
		t.Fatal("no write was refused with the disk full")
	}
	exec(`DROP TABLE crab_proof_filler`)
	retry("written after space was freed")
	if facts("written after space was freed") != 1 {
		t.Fatal("record count wrong after space was freed")
	}
	srv.stop()
	e.oracle("after outages")
}

// T-242 (R-14): a backup of a populated instance restores onto a replacement
// database with every table, projection and supporting state alike, and every
// blob hash identical.
func TestProofBackupRestorePopulated(t *testing.T) {
	e := newE2E(t)
	alice, bob := seedDemo(t, e)
	// Volume through the ledger only: the board stress fixture also writes
	// projection-only rows, which a restore correctly rebuilds away.
	for i := 0; i < 25; i++ {
		e.post("/incident/INC-014/action", alice, url.Values{"title": {fmt.Sprintf("Restore volume action %02d", i)}, "owner": {"bob"}, "priority": {"P2"}, "source_ref": {"proof"}})
		e.post("/incident/INC-014/uncertainty", alice, url.Values{"kind": {"unknown"}, "statement": {fmt.Sprintf("Restore volume unknown %02d", i)}})
	}
	e.approveAll(bob)
	e.onboard(alice, bob, "jane", "Minor", true)
	e.issueIntakeKey(alice, bob, "siem-backup", "alice", e.now().Add(24*time.Hour))
	e.recordIndicator(alice, "INC-014", "GENERAL", "ip", "203.0.113.7")
	e.post("/incident/INC-014/decision", alice, url.Values{"decision": {"Pending at backup time"}, "rationale": {"r"}})
	ctx := context.Background()
	before := hashTables(t, e.admin)
	if len(before) < 40 {
		t.Fatalf("only %d tables fingerprinted", len(before))
	}

	bin := proofBinary(t)
	dir := t.TempDir()
	dump := filepath.Join(dir, "populated.dump")
	if out, err := exec.Command(bin, "backup", "--output", dump).CombinedOutput(); err != nil {
		t.Fatalf("backup: %v\n%s", err, out)
	}

	// A replacement database on the same server, empty.
	adminURL := os.Getenv("CRAB_TEST_POSTGRES_ADMIN_URL")
	_, _ = e.admin.Exec(ctx, `DROP DATABASE IF EXISTS crab_restore WITH (FORCE)`)
	if _, err := e.admin.Exec(ctx, `CREATE DATABASE crab_restore OWNER `+e2eOwnerRole); err != nil {
		t.Fatal(err)
	}
	serverAdmin := e.admin
	defer serverAdmin.Exec(ctx, `DROP DATABASE IF EXISTS crab_restore WITH (FORCE)`)
	restoreAdminURL := strings.Replace(adminURL, "/crab_test", "/crab_restore", 1)
	ownerURL := strings.Replace(restoreAdminURL, "postgres:postgres", e2eOwnerRole+":owner-e2e-password", 1)
	runtimeURL := strings.Replace(restoreAdminURL, "postgres:postgres", e2eRuntimeRole+":runtime-e2e-password", 1)
	cmd := exec.Command(bin, "restore", "--input", dump)
	cmd.Env = append(os.Environ(), "CRAB_MAINTENANCE_DATABASE_URL="+ownerURL, "CRAB_DATABASE_URL="+runtimeURL)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("restore: %v\n%s", err, out)
	}
	restored, err := postgres.OpenPool(ctx, restoreAdminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	after := hashTables(t, restored)
	var diffs []string
	for table, sum := range before {
		if after[table] != sum {
			diffs = append(diffs, table)
		}
	}
	for table := range after {
		if _, ok := before[table]; !ok {
			diffs = append(diffs, table+" (new)")
		}
	}
	if len(diffs) > 0 {
		t.Fatalf("restored tables differ: %s", strings.Join(diffs, ", "))
	}
	blobs, err := postgres.NewBlobStore(restored, 64<<20)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := restored.Query(ctx, `SELECT blob_id FROM crab.blobs`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) == 0 {
		t.Fatal("the populated fixture has no blobs to check")
	}
	for _, id := range ids {
		if _, err := blobs.Verify(ctx, id); err != nil {
			t.Fatalf("restored blob %s does not re-hash: %v", id, err)
		}
	}
	t.Logf("restored %d tables and %d blobs identically", len(after), len(ids))
}

func hashTables(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema='crab' AND table_type='BASE TABLE' ORDER BY table_name`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		_ = rows.Scan(&name)
		tables = append(tables, name)
	}
	rows.Close()
	out := map[string]string{}
	for _, table := range tables {
		var sum *string
		if err := pool.QueryRow(ctx, `SELECT md5(string_agg(t::text, '|' ORDER BY t::text)) FROM crab.`+table+` t`).Scan(&sum); err != nil {
			t.Fatal(err)
		}
		out[table] = "empty"
		if sum != nil {
			out[table] = *sum
		}
	}
	return out
}

// buildOldRelease builds the remediation baseline (66b188a) and a helper that
// applies its migrations, as that release's own test harness did.
func buildOldRelease(t *testing.T) (crab, migrate string) {
	t.Helper()
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("git: %v", err)
	}
	repo := strings.TrimSpace(string(root))
	wt := filepath.Join(t.TempDir(), "old")
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "--detach", wt, "66b188a").CombinedOutput(); err != nil {
		t.Fatalf("worktree: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("git", "-C", repo, "worktree", "remove", "--force", wt).Run() })
	if work, err := os.ReadFile(filepath.Join(repo, "go.work")); err == nil { // offline development containers
		_ = os.WriteFile(filepath.Join(wt, "go.work"), work, 0o644)
	}
	helper := filepath.Join(wt, "cmd", "proofmigrate")
	_ = os.MkdirAll(helper, 0o755)
	if err := os.WriteFile(filepath.Join(helper, "main.go"), []byte(`package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
)

func main() {
	owner, err := postgres.OpenPool(context.Background(), os.Args[1])
	if err == nil {
		err = postgres.Migrate(context.Background(), owner, os.Args[2])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	crab, migrate = filepath.Join(wt, "crab-old"), filepath.Join(wt, "migrate-old")
	for out, pkg := range map[string]string{crab: "./cmd/crab", migrate: "./cmd/proofmigrate"} {
		cmd := exec.Command("go", "build", "-o", out, pkg)
		cmd.Dir = wt
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build old %s: %v\n%s", pkg, err, b)
		}
	}
	return crab, migrate
}

func (c *client) get(path string) string {
	res, err := c.http.Get(c.base + path)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return string(b)
}

// confirmAll confirms every proposal on this person's Approvals page.
func (c *client) confirmAll() int {
	n := 0
	for _, f := range approvalForm.FindAllString(c.get("/approvals"), -1) {
		form := url.Values{}
		for _, m := range hiddenField.FindAllStringSubmatch(f, -1) {
			form.Set(m[1], htmlUnescape(m[2]))
		}
		if form.Get("record_id") == "" {
			continue
		}
		form.Set("outcome", "confirmed")
		if res, err := c.form("/decide", form); err == nil && res.StatusCode == http.StatusSeeOther {
			n++
		}
	}
	return n
}

// T-243 (R-15): an instance populated by the baseline release (66b188a)
// migrates to this release, verifies and continues; an interrupted migration
// leaves the old schema; a failed migration refuses clearly and changes
// nothing.
func TestProofUpgradeFromBaseline(t *testing.T) {
	e := newE2E(t)
	oldCrab, oldMigrate := buildOldRelease(t)
	newCrab := proofBinary(t)
	ctx := context.Background()

	adminURL := strings.Replace(os.Getenv("CRAB_TEST_POSTGRES_ADMIN_URL"), "/crab_test", "/crab_upgrade", 1)
	_, _ = e.admin.Exec(ctx, `DROP DATABASE IF EXISTS crab_upgrade WITH (FORCE)`)
	if _, err := e.admin.Exec(ctx, `CREATE DATABASE crab_upgrade OWNER `+e2eOwnerRole); err != nil {
		t.Fatal(err)
	}
	serverAdmin := e.admin
	t.Cleanup(func() {
		_, _ = serverAdmin.Exec(context.Background(), `DROP DATABASE IF EXISTS crab_upgrade WITH (FORCE)`)
	})
	ownerURL := strings.Replace(adminURL, "postgres:postgres", e2eOwnerRole+":owner-e2e-password", 1)
	runtimeURL := strings.Replace(adminURL, "postgres:postgres", e2eRuntimeRole+":runtime-e2e-password", 1)
	t.Setenv("CRAB_DATABASE_URL", runtimeURL)
	t.Setenv("CRAB_MAINTENANCE_DATABASE_URL", ownerURL)
	upgrade, err := postgres.OpenPool(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer upgrade.Close()
	e.admin = upgrade // the oracle and counts now look at crab_upgrade
	version := func() int {
		t.Helper()
		v, err := postgres.SchemaVersion(ctx, upgrade)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	// The baseline release creates and populates the instance.
	if out, err := exec.Command(oldMigrate, ownerURL, e2eRuntimeRole).CombinedOutput(); err != nil {
		t.Fatalf("baseline migrate: %v\n%s", err, out)
	}
	if v := version(); v != 17 {
		t.Fatalf("baseline schema version %d, want 17", v)
	}
	old := startCrab(t, oldCrab, "")
	anon := &client{t: t, base: old.base, http: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	if res, err := anon.form("/setup", url.Values{"prepared_by": {"alice"}, "prepared_password": {e2eAlicePassword}, "confirmed_by": {"bob"}, "confirmed_password": {e2eBobPassword}}); err != nil || res.StatusCode != http.StatusSeeOther {
		t.Fatalf("baseline setup: %v %v\n%s", err, res, old.log)
	}
	alice := signInTo(t, old.base, "alice", e2eAlicePassword)
	bob := signInTo(t, old.base, "bob", e2eBobPassword)
	if res, err := alice.form("/incident", url.Values{"incident_id": {"INC-UP"}, "mode": {"live"}, "title": {"Upgrade night"}, "summary": {"Incident running across the upgrade"}, "severity": {"Severity level 3"}}); err != nil || res.StatusCode != http.StatusSeeOther {
		t.Fatalf("baseline declare: %v %v", err, res)
	}
	if bob.confirmAll() == 0 {
		t.Fatalf("baseline: nothing to confirm\n%s", old.log)
	}
	if res, err := alice.form("/incident/INC-UP/fact", url.Values{"statement": {"Recorded on the baseline release"}, "source": {"proof"}}); err != nil || res.StatusCode != http.StatusSeeOther {
		t.Fatalf("baseline fact: %v %v", err, res)
	}
	old.stop()
	entries := e.ledgerCount()

	// An interrupted migration leaves the old schema, and the old release
	// still starts on it; this release refuses with a clear message.
	cmd := exec.Command(newCrab, "migrate")
	cmd.Env = append(os.Environ(), "CRAB_FAILPOINT=migration-before-commit")
	_ = cmd.Run()
	if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); !ok || !status.Signaled() {
		t.Fatal("migration was not killed at the failpoint")
	}
	if v := version(); v != 17 {
		t.Fatalf("interrupted migration left schema version %d", v)
	}
	refused, _ := exec.Command(newCrab, "serve").CombinedOutput()
	if !strings.Contains(string(refused), "run `crab migrate`") {
		t.Fatalf("this release did not refuse the old schema clearly:\n%s", refused)
	}
	startCrab(t, oldCrab, "").stop()

	// A failed migration refuses clearly and changes nothing.
	if _, err := upgrade.Exec(ctx, `SET ROLE `+e2eOwnerRole+`; CREATE VIEW crab.intake_keys AS SELECT 1 AS x; RESET ROLE`); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(newCrab, "migrate").CombinedOutput()
	if err == nil || !strings.Contains(string(out), "migration failed; the schema is unchanged at version 17") {
		t.Fatalf("failed migration was not refused clearly: %v\n%s", err, out)
	}
	if v := version(); v != 17 || e.ledgerCount() != entries {
		t.Fatalf("failed migration changed state: version %d, ledger %d→%d", v, entries, e.ledgerCount())
	}
	if _, err := upgrade.Exec(ctx, `DROP VIEW crab.intake_keys`); err != nil {
		t.Fatal(err)
	}

	// The real migration, then verification and the incident continues.
	out, err = exec.Command(newCrab, "migrate").CombinedOutput()
	if err != nil || !strings.Contains(string(out), `"to_version": 21`) || !strings.Contains(string(out), `"ledger_verified": true`) {
		t.Fatalf("migrate: %v\n%s", err, out)
	}
	if e.ledgerCount() != entries {
		t.Fatal("migration changed the ledger")
	}
	e.oracle("after migration")
	srv := startCrab(t, newCrab, "")
	alice = signInTo(t, srv.base, "alice", e2eAlicePassword)
	if board := alice.get("/incident/INC-UP"); !strings.Contains(board, "Recorded on the baseline release") {
		t.Fatal("the migrated Board lost the baseline fact")
	}
	if res, err := alice.form("/incident/INC-UP/fact", url.Values{"statement": {"Recorded after the upgrade"}, "source": {"proof"}}); err != nil || res.StatusCode != http.StatusSeeOther {
		t.Fatalf("fact after upgrade: %v %v\n%s", err, res, srv.log)
	}
	srv.stop()
	e.oracle("after continuing")
}
