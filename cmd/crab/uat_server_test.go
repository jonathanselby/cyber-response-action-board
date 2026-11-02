package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestUATServe runs a seeded CRAB for the browser and accessibility harness
// in uat/ (R-17, R-18). It is test-only: it never runs in the normal suite.
// It seeds the demo data and the command-board stress fixture, serves CRAB on
// loopback, and serves a small control API the harness uses to observe and
// arrange state (ledger count, row counts, ending a person's sessions). It
// writes the addresses to CRAB_UAT_SERVE_FILE and stops on POST /stop or
// after CRAB_UAT_SERVE_SECONDS.
func TestUATServe(t *testing.T) {
	file := os.Getenv("CRAB_UAT_SERVE_FILE")
	if file == "" {
		t.Skip("CRAB_UAT_SERVE_FILE not set")
	}
	lifetime := 30 * time.Minute
	if s, err := strconv.Atoi(os.Getenv("CRAB_UAT_SERVE_SECONDS")); err == nil && s > 0 {
		lifetime = time.Duration(s) * time.Second
	}
	e := newE2E(t)
	alice, bob := seedDemo(t, e)
	e.seedBoardStress(alice, bob)

	ctx := context.Background()
	app, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctl, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	countOf := map[string]string{
		"ledger":   `SELECT count(*) FROM crab.ledger_entries`,
		"evidence": `SELECT count(*) FROM crab.evidence_register`,
		"blobs":    `SELECT count(*) FROM crab.blobs`,
		"facts":    `SELECT count(*) FROM crab.incident_facts`,
		"sessions": `SELECT count(*) FROM crab.sessions`,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /count/{what}", func(w http.ResponseWriter, r *http.Request) {
		q, ok := countOf[r.PathValue("what")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		var n int
		if err := e.admin.QueryRow(ctx, q).Scan(&n); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]int{"count": n})
	})
	mux.HandleFunc("POST /expire/{human}", func(w http.ResponseWriter, r *http.Request) {
		if _, err := e.admin.Exec(ctx, `DELETE FROM crab.sessions WHERE human_id=$1`, r.PathValue("human")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /stop", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		select {
		case <-stop:
		default:
			close(stop)
		}
	})
	appSrv := &http.Server{Handler: e.handler, ReadHeaderTimeout: 10 * time.Second}
	ctlSrv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = appSrv.Serve(app) }()
	go func() { _ = ctlSrv.Serve(ctl) }()
	info := map[string]any{
		"base":     "http://" + app.Addr().String(),
		"control":  "http://" + ctl.Addr().String(),
		"users":    map[string]string{"alice": e2eAlicePassword, "bob": e2eBobPassword},
		"incident": "INC-014",
		"stress":   stressIncident,
	}
	raw, _ := json.Marshal(info)
	if err := os.WriteFile(file, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("UAT CRAB at %s, control at %s", info["base"], info["control"])
	select {
	case <-stop:
	case <-time.After(lifetime):
		t.Log("UAT server lifetime ended")
	}
	shutdown, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, s := range []*http.Server{appSrv, ctlSrv} {
		if err := s.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Log(err)
		}
	}
}
