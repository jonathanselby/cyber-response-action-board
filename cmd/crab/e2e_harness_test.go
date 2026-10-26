package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
)

// e2e drives the real operator runtime and browser routes against PostgreSQL.
// It exists so leakage and governance rules are proved through the same path
// a responder uses, not only through fakes.
type e2e struct {
	t       *testing.T
	handler http.Handler
	export  string
}

const (
	e2eOwnerRole   = "crab_owner_e2e"
	e2eRuntimeRole = "crab_runtime_e2e"
)

func newE2E(t *testing.T) *e2e {
	t.Helper()
	adminURL := os.Getenv("CRAB_TEST_POSTGRES_ADMIN_URL")
	if adminURL == "" {
		t.Skip("CRAB_TEST_POSTGRES_ADMIN_URL not set")
	}
	ctx := context.Background()
	// A separate database keeps these tests independent of the data-tier
	// integration tests, which run in parallel against crab_test.
	bootstrapAdmin, err := postgres.OpenPool(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err := bootstrapAdmin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname='crab_e2e')`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		if _, err := bootstrapAdmin.Exec(ctx, `CREATE DATABASE crab_e2e`); err != nil {
			t.Fatal(err)
		}
	}
	bootstrapAdmin.Close()
	adminURL = strings.Replace(adminURL, "/crab_test", "/crab_e2e", 1)
	admin, err := postgres.OpenPool(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		_, _ = admin.Exec(ctx, `DROP SCHEMA IF EXISTS crab CASCADE`)
		for _, role := range []string{e2eRuntimeRole, e2eOwnerRole} {
			_, _ = admin.Exec(ctx, `REVOKE ALL PRIVILEGES ON DATABASE crab_e2e FROM `+role)
			_, _ = admin.Exec(ctx, `DROP ROLE IF EXISTS `+role)
		}
	}
	cleanup()
	for _, stmt := range []string{
		`CREATE ROLE ` + e2eOwnerRole + ` LOGIN PASSWORD 'owner-e2e-password'`,
		`CREATE ROLE ` + e2eRuntimeRole + ` LOGIN PASSWORD 'runtime-e2e-password'`,
		`GRANT CREATE ON DATABASE crab_e2e TO ` + e2eOwnerRole,
	} {
		if _, err := admin.Exec(ctx, stmt); err != nil {
			cleanup()
			admin.Close()
			t.Fatal(err)
		}
	}
	ownerURL := strings.Replace(adminURL, "postgres:postgres", e2eOwnerRole+":owner-e2e-password", 1)
	runtimeURL := strings.Replace(adminURL, "postgres:postgres", e2eRuntimeRole+":runtime-e2e-password", 1)
	owner, err := postgres.OpenPool(ctx, ownerURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(ctx, owner, e2eRuntimeRole); err != nil {
		t.Fatal(err)
	}
	owner.Close()

	dir := t.TempDir()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	writePrivate := func(name string, v any) string {
		raw, _ := json.Marshal(v)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	keyring := writePrivate("keyring.json", map[string]string{"server": base64.StdEncoding.EncodeToString(pub)})
	signing := writePrivate("signing.json", signingKeyFile{KeyID: "server", PrivateKey: base64.StdEncoding.EncodeToString(priv)})
	export := filepath.Join(dir, "export")
	if err := os.MkdirAll(export, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CRAB_CONFIG_FILE", "")
	t.Setenv("CRAB_DATABASE_URL", runtimeURL)
	t.Setenv("CRAB_LEDGER_KEYRING_FILE", keyring)
	t.Setenv("CRAB_SERVER_SIGNING_KEY_FILE", signing)
	t.Setenv("CRAB_EXPORT_DIR", export)
	t.Setenv("CRAB_LISTEN_ADDR", "127.0.0.1:0")
	server, err := web.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup()
		admin.Close()
	})
	return &e2e{t: t, handler: server.Handler(), export: export}
}

func (e *e2e) do(method, path, cookie string, form url.Values) *httptest.ResponseRecorder {
	e.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, body)
	r.RemoteAddr = "127.0.0.1:50000"
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: "crab_session", Value: cookie})
	}
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, r)
	return w
}

func (e *e2e) get(path, cookie string) string {
	e.t.Helper()
	w := e.do(http.MethodGet, path, cookie, nil)
	if w.Code != http.StatusOK {
		e.t.Fatalf("GET %s code=%d body=%s", path, w.Code, firstError(w.Body.String()))
	}
	return w.Body.String()
}

// post submits a form and requires the post-redirect-get success path.
func (e *e2e) post(path, cookie string, form url.Values) {
	e.t.Helper()
	w := e.do(http.MethodPost, path, cookie, form)
	if w.Code != http.StatusSeeOther {
		e.t.Fatalf("POST %s code=%d error=%s", path, w.Code, firstError(w.Body.String()))
	}
}

// refused submits a form and requires a refusal.
func (e *e2e) refused(path, cookie string, form url.Values) string {
	e.t.Helper()
	w := e.do(http.MethodPost, path, cookie, form)
	if w.Code == http.StatusSeeOther {
		e.t.Fatalf("POST %s unexpectedly succeeded", path)
	}
	return w.Body.String()
}

var errorSummary = regexp.MustCompile(`(?s)<div class="gas-error-summary".*?<p>(.*?)</p>`)

func firstError(body string) string {
	if m := errorSummary.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	if len(body) > 300 {
		return body[:300]
	}
	return body
}

const (
	e2eAlicePassword = "Alice-Password-1!"
	e2eBobPassword   = "Bob-Password-12!"
)

// bootstrap runs the two-person setup and returns alice's and bob's sessions.
func (e *e2e) bootstrap() (alice, bob string) {
	e.t.Helper()
	e.post("/setup", "", url.Values{"prepared_by": {"alice"}, "prepared_password": {e2eAlicePassword}, "confirmed_by": {"bob"}, "confirmed_password": {e2eBobPassword}})
	return e.login("alice", e2eAlicePassword), e.login("bob", e2eBobPassword)
}

func (e *e2e) login(id, password string) string {
	e.t.Helper()
	w := e.do(http.MethodPost, "/login", "", url.Values{"human_id": {id}, "password": {password}})
	for _, c := range w.Result().Cookies() {
		if c.Name == "crab_session" && c.Value != "" {
			return c.Value
		}
	}
	e.t.Fatalf("login %s failed: code=%d %s", id, w.Code, firstError(w.Body.String()))
	return ""
}

var approvalForm = regexp.MustCompile(`(?s)<form[^>]*action="/decide".*?</form>`)
var hiddenField = regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`)

// approveAll confirms every proposal waiting for this person.
func (e *e2e) approveAll(cookie string) int {
	e.t.Helper()
	page := e.get("/approvals", cookie)
	count := 0
	seen := map[string]bool{}
	for _, f := range approvalForm.FindAllString(page, -1) {
		form := url.Values{}
		for _, m := range hiddenField.FindAllStringSubmatch(f, -1) {
			form.Set(m[1], htmlUnescape(m[2]))
		}
		key := form.Get("record_id") + form.Get("version")
		if form.Get("record_id") == "" || seen[key] {
			continue
		}
		seen[key] = true
		form.Set("outcome", "confirmed")
		e.post("/decide", cookie, form)
		count++
	}
	return count
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&#34;", `"`, "&#39;", "'", "&#43;", "+")
	return r.Replace(s)
}

// declare proposes an incident as maker; confirmer approves it.
func (e *e2e) declare(maker, confirmer, id, title, severity string) {
	e.t.Helper()
	e.post("/incident", maker, url.Values{"incident_id": {id}, "mode": {"exercise"}, "title": {title}, "summary": {title + " summary"}, "severity": {severity}})
	if n := e.approveAll(confirmer); n == 0 {
		e.t.Fatalf("declaration %s had nothing to approve", id)
	}
}
