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
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "Cache-Control"} {
		if w.Header().Get(h) == "" {
			t.Fatalf("missing %s", h)
		}
	}
	if got := w.Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Fatalf("Referrer-Policy = %q; want same-origin", got)
	}
	if !strings.Contains(body, "operator runtime is not configured") {
		t.Fatalf("unconfigured host should explain configuration: %q", body)
	}
}

func TestStaticAssetIsEmbedded(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"/static/crab.css":      "--gas-accent",
		"/_app-shell/shell.css": ".gas-nav",
		"/static/icon.svg":      "<svg",
		"/manifest.webmanifest": `"short_name": "CRAB"`,
		"/service-worker.js":    "PRECACHE",
		"/offline":              "CRAB cannot be reached",
	} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		res := w.Result()
		b, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusOK || !strings.Contains(string(b), want) {
			t.Fatalf("%s unavailable or wrong: %d", path, res.StatusCode)
		}
	}
}
