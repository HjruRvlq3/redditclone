package server

import (
	"context"
	"net/http"

	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/jwtconfig"
	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/model"
	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/store"
)

type ctxKey struct{}

var userKey ctxKey

// requireAuth ...
func requireAuth(jwtManager *jwtconfig.Manager, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

	}
}

// userFromContext ...
func userFromContext(ctx context.Context) *model.User {

}

// registerSecurityHandlers ...
func registerSecurityHandlers(mux *http.ServeMux, postStore *store.PostStore, jwtManager *jwtconfig.Manager) {
	mux.HandleFunc("POST /api/posts/", requireAuth(jwtManager, postPosts(postStore)))
}
