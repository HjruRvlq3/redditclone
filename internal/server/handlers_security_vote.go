package server

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/store"
)

// handleVote upvote/downvote
func handleVote(w http.ResponseWriter, r *http.Request, postStore *store.PostStore, vote int) {
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

	post, err := postStore.Vote(postID, user.Id, vote)
	if err != nil {
		if errors.Is(err, store.ErrPostNotFound) {
			writeJSONError(w, http.StatusNotFound, "post not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "server error")
		return
	}

	writeJSONResponse(w, http.StatusOK, post)
}

// getUpvote рейтинг поста вверх
func getUpvote(postStore *store.PostStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handleVote(w, r, postStore, 1)
	}
}

// getDownvote рейтинг поста вниз
func getDownvote(postStore *store.PostStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handleVote(w, r, postStore, -1)
	}
}

// getUnvote отменить голос пользователя
func getUnvote(postStore *store.PostStore) http.HandlerFunc {
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

		post, err := postStore.Unvote(postID, user.Id)
		if err != nil {
			if errors.Is(err, store.ErrPostNotFound) {
				writeJSONError(w, http.StatusNotFound, "post not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "server error")
			return
		}

		writeJSONResponse(w, http.StatusOK, post)
	}
}
