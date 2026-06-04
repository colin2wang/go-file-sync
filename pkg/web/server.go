// Package web provides an optional HTTP API and dashboard for monitoring
// sync status. It uses only Go's standard library (net/http).
package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go-file-sync/pkg/stats"
)

// Server is the HTTP API and dashboard server.
type Server struct {
	server   *http.Server
	mux      *http.ServeMux
	metrics  *stats.Manager
	pauseFn  func() error
	resumeFn func() error
	reloadFn func() error
	mu       sync.RWMutex
	paused   bool
}

// Config holds web server configuration.
type Config struct {
	Enabled   bool
	Listen    string
	Dashboard bool
}

// New creates a new web server.
func New(cfg Config, metrics *stats.Manager) *Server {
	s := &Server{
		mux:     http.NewServeMux(),
		metrics: metrics,
	}
	s.registerRoutes()
	return s
}

// SetPauseFunc sets the function to pause all sync tasks.
func (s *Server) SetPauseFunc(fn func() error) {
	s.pauseFn = fn
}

// SetResumeFunc sets the function to resume all sync tasks.
func (s *Server) SetResumeFunc(fn func() error) {
	s.resumeFn = fn
}

// SetReloadFunc sets the function to hot-reload configuration.
func (s *Server) SetReloadFunc(fn func() error) {
	s.reloadFn = fn
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/v1/status", s.handleStatus)
	s.mux.HandleFunc("/api/v1/tasks", s.handleTasks)
	s.mux.HandleFunc("/api/v1/metrics", s.handleMetrics)
	s.mux.HandleFunc("/api/v1/pause", s.handlePause)
	s.mux.HandleFunc("/api/v1/resume", s.handleResume)
	s.mux.HandleFunc("/api/v1/reload", s.handleReload)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/", s.handleDashboard)
}

// Start starts the HTTP server in a background goroutine.
func (s *Server) Start(listen string) error {
	s.server = &http.Server{
		Addr:    listen,
		Handler: s.mux,
	}
	return s.server.ListenAndServe()
}

// Stop shuts down the HTTP server gracefully.
func (s *Server) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// --- Handlers ---

type statusResponse struct {
	Uptime  string         `json:"uptime"`
	Paused  bool           `json:"paused"`
	Metrics *stats.Metrics `json:"metrics"`
	Tasks   []taskStatus   `json:"tasks"`
}

type taskStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	paused := s.paused
	s.mu.RUnlock()

	g := s.metrics.Global()
	resp := statusResponse{
		Uptime:  time.Since(g.StartTime).Round(time.Second).String(),
		Paused:  paused,
		Metrics: g,
		Tasks:   []taskStatus{},
	}
	writeJSON(w, resp)
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"info": "task listing available via /api/v1/status"})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"global": s.metrics.Global(),
	})
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	if s.pauseFn != nil {
		if err := s.pauseFn(); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	s.mu.Lock()
	s.paused = true
	s.mu.Unlock()
	writeJSON(w, map[string]string{"status": "paused"})
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	if s.resumeFn != nil {
		if err := s.resumeFn(); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	s.mu.Lock()
	s.paused = false
	s.mu.Unlock()
	writeJSON(w, map[string]string{"status": "resumed"})
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	if s.reloadFn != nil {
		if err := s.reloadFn(); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, map[string]string{"status": "reloaded"})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK\n")
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, dashboardHTML)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>go-file-sync Dashboard</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; background: #0d1117; color: #c9d1d9; }
h1 { color: #58a6ff; }
pre { background: #161b22; padding: 15px; border-radius: 6px; overflow-x: auto; }
.card { background: #161b22; border: 1px solid #30363d; border-radius: 6px; padding: 16px; margin: 16px 0; }
button { background: #238636; color: white; border: none; padding: 8px 16px; border-radius: 6px; cursor: pointer; margin-right: 8px; }
button:hover { background: #2ea043; }
.error { color: #f85149; }
</style>
</head>
<body>
<h1>go-file-sync Dashboard</h1>
<div class="card" id="status">
<h2>Status</h2>
<pre id="status-content">Loading...</pre>
</div>
<div class="card">
<h2>Actions</h2>
<button onclick="action('/api/v1/pause')">Pause</button>
<button onclick="action('/api/v1/resume')">Resume</button>
<button onclick="action('/api/v1/reload')">Reload Config</button>
</div>
<script>
async function refresh() {
  const r = await fetch('/api/v1/status');
  const d = await r.json();
  document.getElementById('status-content').textContent = JSON.stringify(d, null, 2);
}
async function action(path) {
  await fetch(path, { method: 'POST' });
  refresh();
}
setInterval(refresh, 3000);
refresh();
</script>
</body>
</html>`
