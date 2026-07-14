package httpapi

import (
	_ "embed"
	"net/http"
)

// indexHTML — одностраничный веб-клиент, вшитый в бинарник.
//
//go:embed index.html
var indexHTML []byte

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// ServeMux с паттерном "GET /" ловит все пути — отдаём index только для корня.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}
