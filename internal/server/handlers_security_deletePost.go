package server

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/store"
)

// deletePost удаление поста
func deletePost(postStore *store.PostStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r.Context())
		if user == nil {
			writeJSONError(w, http.StatusInternalServerError, "server error")
			return
		}

		postID := r.PathValue("POST_ID")
		if _, err := uuid.Parse(postID); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid post id")
			return
		}

		// читаем пост, чтобы проверить права
		post, err := postStore.GetById(postID)
		if err != nil {
			if errors.Is(err, store.ErrPostNotFound) {
				writeJSONError(w, http.StatusNotFound, "post not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "server error")
			return
		}

		if !user.Admin && (post.Author == nil || post.Author.Id != user.Id) {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// удаляем
		if err := postStore.DeletePost(postID); err != nil {
			if errors.Is(err, store.ErrPostNotFound) {
				writeJSONError(w, http.StatusNotFound, "post not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "server error")
			return
		}

		writeJSONResponse(w, http.StatusOK, map[string]string{"message": "success"})
	}
}
