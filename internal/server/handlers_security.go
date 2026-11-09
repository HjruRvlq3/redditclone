package server

import (
	"context"
	"net/http"
	"strings"

	"redditclone/internal/jwtconfig"
	"redditclone/internal/model"
	"redditclone/internal/store"
)

type ctxKey struct{}

var userKey ctxKey

// middlewareFunc извлекает токен из реквеста
func middlewareFunc(jwtManager *jwtconfig.Manager, nextHandlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		user, err := jwtManager.ParseToken(parts[1])
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), userKey, user)
		nextHandlerFunc(w, r.WithContext(ctx))
	}
}

// userFromContext достает юзера из контекста
func userFromContext(ctx context.Context) *model.User {
	u, _ := ctx.Value(userKey).(*model.User)
	return u
}

// registerSecurityHandlers ...
func registerSecurityHandlers(mux *http.ServeMux, postStore *store.PostStore, jwtManager *jwtconfig.Manager) {
	mux.HandleFunc("POST /api/posts/", middlewareFunc(jwtManager, postPosts(postStore)))
}
