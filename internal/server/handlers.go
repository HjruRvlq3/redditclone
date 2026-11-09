package server

import (
	"encoding/json"
	"log"
	"net/http"

	"redditclone/internal/jwtconfig"
	"redditclone/internal/store"
)

// registerReq ...
type registerReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// writeJSONError формирует отшибку
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"message": msg})
}

// writeJSONResponse формирует ответ
func writeJSONResponse(w http.ResponseWriter, status int, encodeValue interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	err := json.NewEncoder(w).Encode(encodeValue)
	if err != nil {
		log.Printf("encode response: %v", err)
	}
}

// handleConfig настройка роутов
func handleConfig(mux *http.ServeMux, userStore *store.UserStore, postStore *store.PostStore, jwtManager *jwtconfig.Manager) {
	// статика
	registerStaticHandlers(mux)
	// public
	registerPublicHandlers(mux, userStore, postStore, jwtManager)
	// security
	registerSecurityHandlers(mux, postStore, jwtManager)

	// ...
	mux.HandleFunc("/", handleMain())
}

// handleMain обработка "/"
func handleMain() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "./static/html/index.html")
	}
}
