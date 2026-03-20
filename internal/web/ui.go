package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if strings.Contains(path, ".") {
		f, err := distFS.Open("dist/" + path)
		if err == nil {
			f.Close()
			sub, _ := fs.Sub(distFS, "dist")
			http.FileServerFS(sub).ServeHTTP(w, r)
			return
		}
	}
	data, err := distFS.ReadFile("dist/index.html")
	if err != nil {
		http.Error(w, "UI not built. Run: cd frontend && npm run build", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
