package server

import (
	"context"
	"net/http"

	"redditclone/internal/jwtconfig"
	"redditclone/internal/model"
	"redditclone/internal/store"
)

type ctxKey struct{}

var userKey ctxKey

// requireAuth ...
func requireAuth(jwtManager *jwtconfig.Manager, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		token := auth[len(prefix):]

		user, err := jwtManager.ParseToken(token)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), userKey, user)
		next(w, r.WithContext(ctx))
	}
}

// userFromContext ...
func userFromContext(ctx context.Context) *model.User {
	u, _ := ctx.Value(userKey).(*model.User)
	return u
}

// registerSecurityHandlers ...
func registerSecurityHandlers(mux *http.ServeMux, postStore *store.PostStore, jwtManager *jwtconfig.Manager) {
	mux.HandleFunc("POST /api/posts/", requireAuth(jwtManager, postPosts(postStore)))
	mux.HandleFunc("POST /api/posts", requireAuth(jwtManager, postPosts(postStore)))
	mux.HandleFunc("POST /api/post/{POST_ID}", requireAuth(jwtManager, postComment(postStore)))
	mux.HandleFunc("DELETE /api/post/{POST_ID}/{COMMENT_ID}", requireAuth(jwtManager, deleteComment(postStore)))
	mux.HandleFunc("GET /api/post/{POST_ID}/upvote", requireAuth(jwtManager, getUpvote(postStore)))
	mux.HandleFunc("GET /api/post/{POST_ID}/downvote", requireAuth(jwtManager, getDownvote(postStore)))
	mux.HandleFunc("GET /api/post/{POST_ID}/unvote", requireAuth(jwtManager, getUnvote(postStore)))
	mux.HandleFunc("DELETE /api/post/{POST_ID}", requireAuth(jwtManager, deletePost(postStore)))
}
