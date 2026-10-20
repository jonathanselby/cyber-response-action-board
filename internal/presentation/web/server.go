package web

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/buildinfo"
)

//go:embed templates/*.html static/*
var assets embed.FS

type Server struct {
	mux      *http.ServeMux
	template *template.Template
	static   http.Handler
	runtime  Runtime
	version  string
}

func New() (*Server, error) {
	t, err := template.ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, err
	}
	s := &Server{template: t, static: http.FileServer(http.FS(staticFS)), version: buildinfo.Version}
	s.mux = http.NewServeMux()
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", s.static))
	s.mux.HandleFunc("GET /healthz", s.health)
	if RuntimeFactory == nil {
		s.mux.HandleFunc("GET /", s.index)
		return s, nil
	}
	runtime, err := RuntimeFactory()
	if err != nil {
		return nil, err
	}
	s.runtime = runtime
	s.registerAPIRoutes()
	s.registerOperatorRoutes()
	return s, nil
}

func (s *Server) Handler() http.Handler { return securityHeaders(s.mux) }

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct{ Version string }{Version: buildinfo.Version}
	if err := s.template.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	status := "presentation-test-shell"
	if s.runtime != nil {
		status = "operator-runtime"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "version": buildinfo.Version, "time": time.Now().UTC().Format(time.RFC3339)})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; img-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
