package server

import (
	"errors"
	"net/http"

	"redditclone/internal/store"
)

// getPostId детали поста с комментами
func getPostId(postStore *store.PostStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// прочитать JSON
		id := r.PathValue("POST_ID")
		if id == "" {
			writeJSONError(w, http.StatusBadRequest, "invalid post id")
			return
		}

		// найти пост
		post, err := postStore.GetByIdAndIncrementViews(id)
		if err != nil {
			if errors.Is(err, store.ErrPostNotFound) {
				writeJSONError(w, http.StatusNotFound, "post not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "server error")
			return
		}

		// отдать пост
		writeJSONResponse(w, http.StatusOK, post)
	}
}
