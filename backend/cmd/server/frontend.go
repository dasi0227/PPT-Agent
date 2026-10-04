package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// withFrontend keeps API, SSE, native file actions and browser storage on one
// stable loopback origin. Only browser routes fall back to the SPA document.
func withFrontend(api http.Handler, directory string) (http.Handler, error) {
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	index := filepath.Join(root, "index.html")
	if info, err := os.Stat(index); err != nil || info.IsDir() {
		return nil, fmt.Errorf("frontend index.html not found in %s", root)
	}
	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		clean := path.Clean("/" + r.URL.Path)
		file := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(clean, "/")))
		if info, err := os.Stat(file); err == nil {
			if info.IsDir() {
				info, err = os.Stat(filepath.Join(file, "index.html"))
			}
			if err == nil && !info.IsDir() {
				w.Header().Set("Cache-Control", "no-cache")
				files.ServeHTTP(w, r)
				return
			}
		}
		if path.Ext(clean) == "" && strings.Contains(r.Header.Get("Accept"), "text/html") {
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, index)
			return
		}
		http.NotFound(w, r)
	}), nil
}
