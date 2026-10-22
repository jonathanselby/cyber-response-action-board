package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShellIsOfflineAndSecurityHeadersArePresent(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Fatal("shell depends on external asset")
	}
	if !strings.Contains(body, "gas-body") || !strings.Contains(body, "/_app-shell/shell.css") {
		t.Fatal("released go-app-shell is not the active presentation chassis")
	}
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "Cache-Control"} {
		if w.Header().Get(h) == "" {
			t.Fatalf("missing %s", h)
		}
	}
	if got := w.Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Fatalf("Referrer-Policy = %q; want same-origin", got)
	}
}

func TestShellAndCRABAssetsAreEmbedded(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "/_app-shell/shell.css", want: ".gas-header"},
		{path: "/static/app.css", want: ".crab-home-layout"},
	} {
		r := httptest.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		res := w.Result()
		b, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if res.StatusCode != http.StatusOK || !strings.Contains(string(b), tc.want) {
			t.Fatalf("embedded asset %s unavailable: %d", tc.path, res.StatusCode)
		}
	}
}
