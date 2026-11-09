package server

import (
	"net/http"

	"redditclone/internal/store"
)

// postPosts добавление поста
func postPosts(postStore *store.PostStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	}
}
