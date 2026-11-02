package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

var (
	resetForm    = regexp.MustCompile(`(?s)<form[^>]*action="/access/reset/confirm".*?</form>`)
	approvalCard = regexp.MustCompile(`(?s)<article class="gas-panel gas-stack" id=.*?</article>`)
	shownCode    = regexp.MustCompile(`<code>([A-Z2-7]{5}-[A-Z2-7]{5}-[A-Z2-7]{5}-[A-Z2-7]{5})</code>`)
	totpSecret   = regexp.MustCompile(`<code>([A-Z2-7]{32})</code>`)
	sessionName  = "crab_session"
)

func e2eTOTP(t *testing.T, secret string) string {
	t.Helper()
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(time.Now().Unix()/30))
	mac := hmac.New(sha1.New, raw)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[off:off+4])&0x7fffffff)%1000000)
}

// confirmReset confirms the pending enrolment code for humanID on the
// confirmer's Approvals page and returns the code shown once.
func (e *e2e) confirmReset(confirmer, humanID string) string {
	e.t.Helper()
	page := e.get("/approvals", confirmer)
	var f string
	for _, card := range approvalCard.FindAllString(page, -1) {
		if strings.Contains(card, "enrolment code to "+humanID) {
			f = resetForm.FindString(card)
		}
	}
	if f == "" {
		e.t.Fatalf("no enrolment code for %s waiting in Approvals", humanID)
	}
	form := url.Values{}
	for _, m := range hiddenField.FindAllStringSubmatch(f, -1) {
		form.Set(m[1], htmlUnescape(m[2]))
	}
	w := e.do(http.MethodPost, "/access/reset/confirm", confirmer, form)
	if w.Code != http.StatusOK {
		e.t.Fatalf("confirm reset code=%d %s", w.Code, firstError(w.Body.String()))
	}
	m := shownCode.FindStringSubmatch(w.Body.String())
	if m == nil {
		e.t.Fatal("enrolment code not shown to the confirmer")
	}
	return m[1]
}

// T-221: the 02:17 journey from the remediation brief, end to end through the
// browser runtime against PostgreSQL.
func TestJaneJourneyAt0217(t *testing.T) {
	e := newE2E(t)
	alice, bob := e.bootstrap()
	e.declare(alice, bob, "INC-004", "Payment platform intrusion", "Severity level 4")
	e.declare(bob, alice, "INC-005", "Unrelated phishing", "Severity level 2")

	// 1–3. A Custodian creates Jane as an external Minor; another confirms;
	// an enrolment code is issued through two Custodians and shown once.
	expiry := time.Now().UTC().Add(7 * 24 * time.Hour).Format("2006-01-02T15:04")
	e.post("/access/person", alice, url.Values{"human_id": {"jane"}, "display_name": {"Jane Doe"}, "organisation": {"Forensic Co"}, "role": {"Minor"}, "external": {"true"}, "account_expires_at": {expiry}})
	approvals := e.get("/approvals", bob)
	if !strings.Contains(approvals, "New person · jane") || !strings.Contains(approvals, "EXTERNAL") {
		t.Fatal("new-person approvals not grouped and worded for the confirmer")
	}
	if n := e.approveAll(bob); n != 2 {
		t.Fatalf("approved %d new-person proposals, want identity and role", n)
	}
	e.post("/access/person/reset", alice, url.Values{"human_id": {"jane"}, "reason": {"new forensic specialist"}})
	if strings.Contains(e.get("/approvals", alice), `action="/access/reset/confirm"`) {
		t.Fatal("the maker was offered their own enrolment code")
	}
	code := e.confirmReset(bob, "jane")

	// 4–6. Sponsor assigns Jane to INC-004 for six hours; FORENSICS read-in.
	sixHours := time.Now().UTC().Add(6 * time.Hour).Format("2006-01-02T15:04")
	e.post("/incident/INC-004/access/member", alice, url.Values{"human_id": {"jane"}, "active": {"true"}, "expires_at": {sixHours}, "reason": {"forensic imaging"}})
	e.approveAll(bob)
	e.post("/incident/INC-004/access/controller", alice, url.Values{"human_id": {"alice"}, "compartment_name": {"FORENSICS"}, "active": {"true"}})
	e.approveAll(bob)
	e.post("/incident/INC-004/access/compartment-member", alice, url.Values{"human_id": {"jane"}, "compartment_name": {"FORENSICS"}, "active": {"true"}, "expires_at": {sixHours}, "reason": {"forensic imaging"}})
	e.approveAll(bob)

	// 7. Jane authenticates: enrolment code, authenticator, own password.
	w := e.do(http.MethodPost, "/login", "", url.Values{"human_id": {"jane"}, "password": {code}})
	if w.Code != http.StatusSeeOther || !strings.HasPrefix(w.Header().Get("Location"), "/enrol") {
		t.Fatalf("enrolment code at sign-in: code=%d location=%q", w.Code, w.Header().Get("Location"))
	}
	begin := e.do(http.MethodPost, "/enrol/begin", "", url.Values{"human_id": {"jane"}, "code": {strings.ToLower(code)}})
	secret := totpSecret.FindStringSubmatch(begin.Body.String())
	if begin.Code != http.StatusOK || secret == nil {
		t.Fatalf("external enrolment did not offer an authenticator: code=%d", begin.Code)
	}
	done := e.do(http.MethodPost, "/enrol/complete", "", url.Values{"human_id": {"jane"}, "code": {code}, "password": {"Forensic-Night-26!"}, "totp": {e2eTOTP(t, secret[1])}})
	var jane string
	for _, c := range done.Result().Cookies() {
		if c.Name == sessionName {
			jane = c.Value
		}
	}
	if done.Code != http.StatusSeeOther || jane == "" {
		t.Fatalf("enrolment did not sign Jane in: code=%d %s", done.Code, firstError(done.Body.String()))
	}
	// External sign-in without the authenticator is refused.
	if w := e.do(http.MethodPost, "/login", "", url.Values{"human_id": {"jane"}, "password": {"Forensic-Night-26!"}}); w.Code != http.StatusUnauthorized {
		t.Fatalf("external sign-in without TOTP code=%d", w.Code)
	}

	// 8. Jane sees INC-004 and FORENSICS.
	if !strings.Contains(e.get("/incident/INC-004", jane), "Payment platform intrusion") {
		t.Fatal("Jane cannot see INC-004")
	}
	if w := e.do(http.MethodGet, "/incident/INC-004/evidence?compartment=FORENSICS", jane, nil); w.Code != http.StatusOK {
		t.Fatalf("Jane cannot reach FORENSICS: code=%d", w.Code)
	}
	// 9. Nothing else: not INC-005, not another compartment, not on her home page.
	if w := e.do(http.MethodGet, "/incident/INC-005", jane, nil); w.Code == http.StatusOK || strings.Contains(w.Body.String(), "Unrelated phishing") {
		t.Fatal("Jane can see INC-005")
	}
	if w := e.do(http.MethodGet, "/incident/INC-004/evidence?compartment=LEGAL", jane, nil); w.Code == http.StatusOK {
		t.Fatal("Jane can reach a compartment she is not read into")
	}
	if home := e.get("/", jane); strings.Contains(home, "INC-005") || strings.Contains(home, "Unrelated phishing") {
		t.Fatal("Jane's home page names INC-005")
	}

	// 10. Immediate revocation: suspension ends her live session at once.
	e.post("/access/person/status", alice, url.Values{"human_id": {"jane"}, "status": {"suspended"}, "reason": {"engagement paused"}})
	e.approveAll(bob)
	if w := e.do(http.MethodGet, "/incident/INC-004", jane, nil); w.Code == http.StatusOK {
		t.Fatalf("suspended Jane still reaches INC-004: code=%d", w.Code)
	}
	if w := e.do(http.MethodPost, "/login", "", url.Values{"human_id": {"jane"}, "password": {"Forensic-Night-26!"}, "totp": {e2eTOTP(t, secret[1])}}); w.Code != http.StatusUnauthorized {
		t.Fatalf("suspended Jane signed in: code=%d", w.Code)
	}
}

// T-249: readiness warns, with the count, when fewer than three Custodians are
// usable; it never blocks operation.
func TestReadinessWarnsBelowThreeUsableCustodians(t *testing.T) {
	e := newE2E(t)
	alice, _ := e.bootstrap()
	page := e.get("/readiness", alice)
	if !strings.Contains(page, "2 usable Custodian(s); at least 3 are recommended") {
		t.Fatal("readiness does not warn about two usable Custodians")
	}
}

// onboard creates a person through the browser: maker proposes, confirmer
// approves identity and role, an enrolment code is confirmed, and the person
// enrols. It returns their session and, for external people, their TOTP key.
func (e *e2e) onboard(maker, confirmer, id, role string, external bool) (session, totpKey string) {
	e.t.Helper()
	form := url.Values{"human_id": {id}, "display_name": {strings.ToUpper(id[:1]) + id[1:]}, "role": {role}, "external": {fmt.Sprint(external)}}
	if external {
		form.Set("account_expires_at", time.Now().UTC().Add(7*24*time.Hour).Format("2006-01-02T15:04"))
	}
	e.post("/access/person", maker, form)
	e.approveAll(confirmer)
	e.post("/access/person/reset", maker, url.Values{"human_id": {id}, "reason": {"onboarding"}})
	code := e.confirmReset(confirmer, id)
	begin := e.do(http.MethodPost, "/enrol/begin", "", url.Values{"human_id": {id}, "code": {code}})
	complete := url.Values{"human_id": {id}, "code": {code}, "password": {"Onboard-" + id + "-26!"}}
	if external {
		m := totpSecret.FindStringSubmatch(begin.Body.String())
		if m == nil {
			e.t.Fatalf("no authenticator key offered to %s", id)
		}
		totpKey = m[1]
		complete.Set("totp", e2eTOTP(e.t, totpKey))
	}
	done := e.do(http.MethodPost, "/enrol/complete", "", complete)
	for _, c := range done.Result().Cookies() {
		if c.Name == sessionName && c.Value != "" {
			return c.Value, totpKey
		}
	}
	e.t.Fatalf("onboarding %s did not sign them in: %s", id, firstError(done.Body.String()))
	return "", ""
}

// signIn signs an onboarded person in again (another browser or device).
func (e *e2e) signIn(id, totpKey string) string {
	e.t.Helper()
	form := url.Values{"human_id": {id}, "password": {"Onboard-" + id + "-26!"}}
	if totpKey != "" {
		form.Set("totp", e2eTOTP(e.t, totpKey))
	}
	w := e.do(http.MethodPost, "/login", "", form)
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionName && c.Value != "" {
			return c.Value
		}
	}
	e.t.Fatalf("sign-in %s failed: %s", id, firstError(w.Body.String()))
	return ""
}

// api calls the JSON API with a bearer token.
func (e *e2e) api(method, path, token, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:50000"
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, r)
	return w
}

// ledgerCount is the number of ledger entries, to prove a refused request
// wrote nothing.
func (e *e2e) ledgerCount() int {
	e.t.Helper()
	var n int
	if err := e.admin.QueryRow(context.Background(), `SELECT count(*) FROM crab.ledger_entries`).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// mcp returns the operator runtime the MCP adapter uses, sharing the browser
// harness's configuration and database.
func (e *e2e) mcp() *operatorRuntime {
	e.t.Helper()
	if e.runtime == nil {
		rt, err := newOperatorRuntime()
		if err != nil {
			e.t.Fatal(err)
		}
		e.runtime = rt.(*operatorRuntime)
	}
	return e.runtime
}

// reaches reports whether a browser session can open an incident page.
func (e *e2e) reaches(cookie, path string) bool {
	e.t.Helper()
	return e.do(http.MethodGet, path, cookie, nil).Code == http.StatusOK
}
