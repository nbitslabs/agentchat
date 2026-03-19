package handler

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

type LandingPageHandler struct {
	indexHTML []byte
	assetFS  http.Handler
}

func NewLandingPageHandler() *LandingPageHandler {
	indexHTML, _ := staticFiles.ReadFile("static/index.html")

	sub, _ := fs.Sub(staticFiles, "static")
	fileServer := http.FileServer(http.FS(sub))

	return &LandingPageHandler{
		indexHTML: indexHTML,
		assetFS:  fileServer,
	}
}

func (h *LandingPageHandler) setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
}

func (h *LandingPageHandler) ServeIndex(w http.ResponseWriter, r *http.Request) {
	h.setSecurityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(h.indexHTML)
}

func (h *LandingPageHandler) ServeAssets(w http.ResponseWriter, r *http.Request) {
	h.setSecurityHeaders(w)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	h.assetFS.ServeHTTP(w, r)
}
