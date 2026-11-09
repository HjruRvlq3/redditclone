package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"redditclone/internal/model"
	"redditclone/internal/store"

	"github.com/google/uuid"
)

type createCommentReq struct {
	Comment string `json:"comment"`
}

// postComment добавление коммента
func postComment(postStore *store.PostStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// получить автора из контекста
		user := userFromContext(r.Context())
		if user == nil {
			writeJSONError(w, http.StatusInternalServerError, "server error")
			return
		}

		// id поста
		postId := r.PathValue("POST_ID")
		if _, err := uuid.Parse(postId); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid post id")
			return
		}

		// получить коммент
		var req createCommentReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json")
			return
		}

		// валидация коммента
		var valErrors []FieldError
		switch {
		case req.Comment == "":
			valErrors = append(valErrors, FieldError{
				Location: "body",
				Param:    "comment",
				Msg:      "is required",
			})
		case len(req.Comment) > 2000:
			valErrors = append(valErrors, FieldError{
				Location: "body",
				Param:    "comment",
				Msg:      "must be at most 2000 characters",
			})
		}

		if len(valErrors) > 0 {
			writeJSONResponse(w, http.StatusUnprocessableEntity, ValidationErrorResponse{Errors: valErrors})
			return
		}

		// создать коммент и добавить к посту
		comment := model.NewComment(user, req.Comment)

		post, err := postStore.PostComment(postId, comment)
		if err != nil {
			if errors.Is(err, store.ErrPostNotFound) {
				writeJSONError(w, http.StatusNotFound, "post not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "server error")
			return
		}

		// отдать новый пост
		writeJSONResponse(w, http.StatusCreated, post)
	}
}
