package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// withStatic serves the built dashboard from webDir for every non-/api
// path, falling back to index.html for client-side routes. This lets the
// native install run as one Go binary with no Node process; the container
// image leaves webDir empty and keeps serving the UI from Node.
func withStatic(webDir string, api http.Handler) http.Handler {
	if webDir == "" {
		return api
	}
	files := http.FileServer(http.Dir(webDir))
	index := filepath.Join(webDir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			api.ServeHTTP(w, r)
			return
		}
		clean := filepath.Join(webDir, filepath.FromSlash(filepath.Clean("/"+r.URL.Path)))
		if info, err := os.Stat(clean); err == nil && !info.IsDir() {
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				// Vite fingerprints everything under /assets.
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	})
}
