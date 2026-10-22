package web

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/buildinfo"
	appshell "github.com/jonathanselby/go-app-shell/shell"
)

//go:embed static/*
var assets embed.FS

type Server struct {
	mux         *http.ServeMux
	static      http.Handler
	shellAssets http.Handler
	renderer    *appshell.Renderer
	runtime     Runtime
	version     string
}

func New() (*Server, error) {
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, err
	}
	shellFS, err := appshell.AssetsFS()
	if err != nil {
		return nil, err
	}
	renderer, err := appshell.New(appshell.Config{
		AssetBase:             "/_app-shell",
		AdditionalStylesheets: []string{"/static/app.css"},
		ThemeColorLight:       "#ffffff",
		ThemeColorDark:        "#101512",
	})
	if err != nil {
		return nil, err
	}
	s := &Server{
		static:      http.FileServer(http.FS(staticFS)),
		shellAssets: http.FileServer(http.FS(shellFS)),
		renderer:    renderer,
		version:     buildinfo.Version,
	}
	s.mux = http.NewServeMux()
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", s.static))
	s.mux.Handle("GET /_app-shell/", http.StripPrefix("/_app-shell/", s.shellAssets))
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
	s.registerDrop2Routes()
	return s, nil
}

func (s *Server) Handler() http.Handler { return securityHeaders(drop2Navigation(s.mux)) }

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	content := template.HTML(`<section class="gas-panel"><div class="gas-stack"><span class="gas-badge gas-badge--info">Build checkpoint</span><h2>CRAB presentation is available</h2><p>The executable shell, signed-ledger verification and security headers are available. Operator capability is enabled only when the durable runtime is configured.</p></div></section>`)
	s.renderShell(w, appshell.Page{
		DocumentTitle: "CRAB",
		App:           appshell.AppHeader{Name: "CRAB", Status: "v" + s.version},
		Title:         "Cyber Response Action Board",
		Summary:       "Incident response that remains legible under pressure and resilient under degraded conditions.",
		Workspace:     appshell.Workspace{Mode: appshell.WorkspaceSingle, Content: content},
	})
}

func (s *Server) renderShell(w http.ResponseWriter, page appshell.Page) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderer.Render(w, page); err != nil {
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
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
