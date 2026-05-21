package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed ui/*
var uiFiles embed.FS

func (s *Server) handleDashboardPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	index, err := uiFiles.ReadFile("ui/index.html")
	if err != nil {
		http.Error(w, "dashboard unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}

func (s *Server) staticAssetsHandler() http.Handler {
	staticFS, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		return http.NotFoundHandler()
	}

	return http.StripPrefix("/static/", http.FileServerFS(staticFS))
}
