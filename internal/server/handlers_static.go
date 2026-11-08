package server

import "net/http"

// registerStaticHandlers ...
func registerStaticHandlers(mux *http.ServeMux) {
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	mux.HandleFunc("GET /favicon.ico", serveFile("./static/html/favicon.ico"))
	mux.HandleFunc("GET /manifest.json", serveFile("./static/html/manifest.json"))
}

// serveFile отдача файлов
func serveFile(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path)
	}
}
