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
}

func TestStaticAssetIsEmbedded(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	res := w.Result()
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || !strings.Contains(string(b), ".shell") {
		t.Fatalf("embedded CSS unavailable: %d", res.StatusCode)
	}
}
