package web

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/buildinfo"
	"github.com/jonathanselby/go-app-shell/shell"
)

//go:embed templates/*.html static/*
var assets embed.FS

const shellAssetBase = "/_app-shell"

type Server struct {
	mux       *http.ServeMux
	shell     *shell.Renderer
	templates *template.Template
	runtime   Runtime
	build     string
	manifest  []byte
	worker    []byte
	offline   []byte
}

// buildVersion identifies this exact build for stylesheet and PWA cache
// invalidation. Developer builds leave Commit as "unknown", so all three
// values are combined.
func buildVersion() string {
	return buildinfo.Version + "-" + buildinfo.Commit + "-" + buildinfo.BuiltAt
}

func New() (*Server, error) {
	build := buildVersion()
	renderer, err := shell.New(shell.Config{
		AssetBase:             shellAssetBase,
		Language:              "en",
		AdditionalStylesheets: []string{"/static/crab.css?v=" + build},
		ThemeColorLight:       "#F7F7F5",
		ThemeColorDark:        "#10161D",
	})
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("crab").Funcs(templateFuncs).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	s := &Server{mux: http.NewServeMux(), shell: renderer, templates: tmpl, build: build}
	if err := s.preparePWA(); err != nil {
		return nil, err
	}

	shellAssets, err := shell.AssetsFS()
	if err != nil {
		return nil, err
	}
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, err
	}
	s.mux.Handle("GET "+shellAssetBase+"/", http.StripPrefix(shellAssetBase+"/", http.FileServer(http.FS(shellAssets))))
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/favicon.png"
		http.FileServer(http.FS(staticFS)).ServeHTTP(w, r)
	})
	s.mux.HandleFunc("GET /manifest.webmanifest", s.serveBytes("application/manifest+json", func() []byte { return s.manifest }))
	s.mux.HandleFunc("GET /service-worker.js", s.serveBytes("text/javascript; charset=utf-8", func() []byte { return s.worker }))
	s.mux.HandleFunc("GET /offline", s.serveBytes("text/html; charset=utf-8", func() []byte { return s.offline }))

	if RuntimeFactory == nil {
		s.mux.HandleFunc("GET /{$}", s.unconfigured)
		return s, nil
	}
	runtime, err := RuntimeFactory()
	if err != nil {
		return nil, err
	}
	s.runtime = runtime
	s.registerAPIRoutes()
	s.registerSystemRoutes()
	s.registerIncidentRoutes()
	s.registerEvidenceRoutes()
	s.registerCommunicationRoutes()
	s.registerAccessRoutes()
	s.registerOrganisationRoutes()
	return s, nil
}

func (s *Server) Handler() http.Handler { return securityHeaders(s.mux) }

func (s *Server) preparePWA() error {
	cfg := shell.PWAConfig{
		Name:            "CRAB — Cyber Response Action Board",
		ShortName:       "CRAB",
		StartURL:        "/",
		Scope:           "/",
		Display:         "standalone",
		ThemeColor:      "#334A5E",
		BackgroundColor: "#F7F7F5",
		Icons: []shell.PWAIcon{
			{Src: "/static/icon.svg", Sizes: "any", Type: "image/svg+xml"},
			{Src: "/static/app-icon-192.png", Sizes: "192x192", Type: "image/png"},
			{Src: "/static/app-icon-512.png", Sizes: "512x512", Type: "image/png"},
		},
		Precache:     []string{shellAssetBase + "/shell.css", shellAssetBase + "/pwa.js", "/static/crab.css?v=" + s.build, "/static/icon.svg"},
		OfflineURL:   "/offline",
		CacheVersion: s.build,
	}
	var err error
	if s.manifest, err = shell.Manifest(cfg); err != nil {
		return err
	}
	if s.worker, err = shell.ServiceWorker(cfg); err != nil {
		return err
	}
	offline, err := s.compose(screen{nav: "offline", title: "CRAB cannot be reached", summary: "Continue on paper.", tmpl: "offline"}, pageData{}, nil)
	if err != nil {
		return err
	}
	s.offline = offline
	return nil
}

func (s *Server) serveBytes(contentType string, body func() []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(body())
	}
}

func (s *Server) unconfigured(w http.ResponseWriter, _ *http.Request) {
	s.page(w, http.StatusOK, screen{title: "CRAB", tmpl: "unconfigured"}, pageData{}, nil)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	status := "shell"
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
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
