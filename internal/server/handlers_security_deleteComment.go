package server

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/model"
	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/store"
)

// deleteComment удаление коммента
func deleteComment(postStore *store.PostStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// получить автора из контекста
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

		commentID := r.PathValue("COMMENT_ID")
		if commentID == "" {
			writeJSONError(w, http.StatusBadRequest, "invalid comment id")
			return
		}

		// получить пост, проверить, что юзер автор или админ
		post, err := postStore.GetById(postID)
		if err != nil {
			if errors.Is(err, store.ErrPostNotFound) {
				writeJSONError(w, http.StatusNotFound, "post not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "server error")
			return
		}

		var comment *model.Comment
		for _, c := range post.Comments {
			if c.Id == commentID {
				comment = c
				break
			}
		}

		if comment == nil {
			writeJSONError(w, http.StatusNotFound, "comment not found")
			return
		}

		if comment.Author == nil || post.Author == nil {
			writeJSONError(w, http.StatusInternalServerError, "server error")
			return
		}

		canDelete := user.Admin ||
			user.Id == comment.Author.Id ||
			user.Id == post.Author.Id

		if !canDelete {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// удаляем комментарий и возвращаем полный обновлённый пост
		updatedPost, err := postStore.DeleteComment(postID, commentID)
		if err != nil {
			switch {
			case errors.Is(err, store.ErrPostNotFound):
				writeJSONError(w, http.StatusNotFound, "post not found")
			case errors.Is(err, store.ErrCommentNotFound):
				writeJSONError(w, http.StatusNotFound, "comment not found")
			default:
				writeJSONError(w, http.StatusInternalServerError, "server error")
			}
			return
		}

		writeJSONResponse(w, http.StatusOK, updatedPost)
	}
}
