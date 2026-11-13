// Package preview serves the media files of the current upload to the GUI so it
// can draw small thumbnails. Only files the upload has reported are served, and
// only when they look like photos or videos.
package preview

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Route is the asset server path handled by Server.Middleware. The file is
// named by the "path" query parameter.
const Route = "/__preview"

// maxAllowed caps the remembered paths; the set starts over when it is full.
const maxAllowed = 20000

var previewableExtensions = map[string]struct{}{
	".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {}, ".webp": {}, ".avif": {}, ".bmp": {},
	".mp4": {}, ".m4v": {}, ".mov": {}, ".webm": {},
}

// Previewable reports whether path has an extension a webview can usually draw.
func Previewable(path string) bool {
	_, ok := previewableExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

// Server remembers which files may be previewed and serves them.
type Server struct {
	mu      sync.RWMutex
	allowed map[string]struct{}
}

func New() *Server {
	return &Server{allowed: make(map[string]struct{})}
}

// Allow marks path as part of an upload, making it available for preview.
func (s *Server) Allow(path string) {
	if path == "" || !Previewable(path) {
		return
	}
	s.mu.RLock()
	_, known := s.allowed[path]
	s.mu.RUnlock()
	if known {
		return
	}

	s.mu.Lock()
	if len(s.allowed) >= maxAllowed {
		s.allowed = make(map[string]struct{})
	}
	s.allowed[path] = struct{}{}
	s.mu.Unlock()
}

func (s *Server) isAllowed(path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.allowed[path]
	return ok
}

// Middleware answers requests for Route and passes everything else to next.
func (s *Server) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != Route {
			next.ServeHTTP(w, r)
			return
		}
		s.serve(w, r)
	})
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Query().Get("path")
	if !s.isAllowed(path) {
		http.NotFound(w, r)
		return
	}

	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "private, max-age=300")
	// ServeContent handles Range requests, so a video frame needs only part of the file.
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), file)
}
