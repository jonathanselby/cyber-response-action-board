package web

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
)

type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newBufferedResponse() *bufferedResponse {
	return &bufferedResponse{header: make(http.Header), status: http.StatusOK}
}
func (b *bufferedResponse) Header() http.Header { return b.header }
func (b *bufferedResponse) WriteHeader(code int) {
	if b.status == http.StatusOK {
		b.status = code
	}
}
func (b *bufferedResponse) Write(p []byte) (int, error) { return b.body.Write(p) }

func drop2Navigation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !drop2NavigationCandidate(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		buffer := newBufferedResponse()
		next.ServeHTTP(buffer, r)
		for key, values := range buffer.header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		body := buffer.body.Bytes()
		if strings.Contains(strings.ToLower(buffer.header.Get("Content-Type")), "text/html") && buffer.status >= 200 && buffer.status < 400 {
			body = injectDrop2Navigation(body, r.URL.Path)
			w.Header().Set("Content-Length", "")
			w.Header().Del("Content-Length")
		}
		w.WriteHeader(buffer.status)
		_, _ = w.Write(body)
	})
}

func drop2NavigationCandidate(path string) bool {
	if strings.HasPrefix(path, "/static/") || path == "/healthz" || strings.HasPrefix(path, "/api/") || strings.HasSuffix(path, "/attachment") {
		return false
	}
	return true
}

func injectDrop2Navigation(body []byte, path string) []byte {
	marker := []byte("<main>")
	if !bytes.Contains(body, marker) {
		return body
	}
	nav := `<nav class="crumb no-print"><a href="/">Incidents</a><span>/</span><a href="/control">Control</a><span>/</span><a href="/readiness">Readiness</a><span>/</span><a href="/access">Access</a><span>/</span><a href="/obligations">Obligation definitions</a></nav>`
	if incidentID := incidentIDFromPath(path); incidentID != "" {
		escaped := url.PathEscape(incidentID)
		nav = `<nav class="crumb no-print"><a href="/">Incidents</a><span>/</span><a href="/control">Control</a><span>/</span><a href="/incident/` + escaped + `">Board</a><span>/</span><a href="/incident/` + escaped + `/operations">Operations</a><span>/</span><a href="/incident/` + escaped + `/access">Access</a><span>/</span><a href="/incident/` + escaped + `/evidence">Evidence</a><span>/</span><a href="/incident/` + escaped + `/obligations">Clocks</a><span>/</span><a href="/incident/` + escaped + `/communications">Communications</a><span>/</span><a href="/incident/` + escaped + `/control">Control</a></nav>`
	}
	return bytes.Replace(body, marker, append(marker, []byte(nav)...), 1)
}

func incidentIDFromPath(path string) string {
	const prefix = "/incident/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	if rest == "" {
		return ""
	}
	if index := strings.IndexByte(rest, '/'); index >= 0 {
		rest = rest[:index]
	}
	value, err := url.PathUnescape(rest)
	if err != nil {
		return ""
	}
	return value
}
