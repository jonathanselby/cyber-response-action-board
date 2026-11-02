package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The intake endpoint only maps the runtime's outcome to HTTP.
func TestFirstReportAPIMapsOutcomes(t *testing.T) {
	srv := withRuntime(t, &fakeRuntime{bootstrapped: true})
	for _, tc := range []struct {
		name, token, body string
		status            int
		want              string
	}{
		{"created", "siem.secret", `{"description":"EDR alert"}`, http.StatusCreated, `"report_id":"REPORT-2"`},
		{"repeat", "siem.secret", `{"description":"EDR alert","external_ref":"dup"}`, http.StatusOK, `"report_id":"REPORT-1"`},
		{"rate", "siem.secret", `{"description":"EDR alert","external_ref":"flood"}`, http.StatusTooManyRequests, "too many requests"},
		{"invalid", "siem.secret", `{"description":""}`, http.StatusBadRequest, "description is required"},
		{"refused", "wrong.secret", `{"description":"EDR alert"}`, http.StatusNotFound, "not found"},
		{"unknown field", "siem.secret", `{"description":"x","attachments":[]}`, http.StatusBadRequest, "invalid JSON"},
	} {
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/v1/first-reports", strings.NewReader(tc.body))
		req.RemoteAddr = "127.0.0.1:5000"
		req.Header.Set("Authorization", "Bearer "+tc.token)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("%s: status %d body %q", tc.name, rec.Code, rec.Body.String())
		}
	}
}
