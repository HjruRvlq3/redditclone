package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"redditclone/internal/model"
	"redditclone/internal/store"
)

type postRequest struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Text     string `json:"text"`
}

// postPosts добавление поста
func postPosts(postStore *store.PostStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// получить юзера
		user := userFromContext(r.Context())

		// получить пост
		var req postRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json")
			return
		}

		// провалидировать пост
		var valErrors []FieldError

		if req.Title == "" || len(req.Title) > 100 || strings.TrimSpace(req.Title) != req.Title {
			valErrors = append(valErrors, FieldError{
				Location: "body",
				Param:    "title",
				Msg:      "must be 1-100 characters long, without leading or trailing spaces",
			})
		}

		if req.Category == "" {
			valErrors = append(valErrors, FieldError{
				Location: "body",
				Param:    "category",
				Msg:      "is required",
			})
		}

		switch req.Type {
		case "link":
			if req.URL == "" {
				valErrors = append(valErrors, FieldError{
					Location: "body",
					Param:    "url",
					Msg:      "is required for link posts",
				})
			} else if u, err := url.Parse(req.URL); err != nil || u.Scheme == "" || u.Host == "" {
				valErrors = append(valErrors, FieldError{
					Location: "body",
					Param:    "url",
					Msg:      "must be a valid absolute url",
				})
			}
		case "text":
			if len(req.Text) < 4 {
				valErrors = append(valErrors, FieldError{
					Location: "body",
					Param:    "text",
					Msg:      "must be at least 4 characters",
				})
			}
		case "":
			valErrors = append(valErrors, FieldError{
				Location: "body",
				Param:    "type",
				Msg:      "is required",
			})
		default:
			valErrors = append(valErrors, FieldError{
				Location: "body",
				Param:    "type",
				Msg:      "must be 'link' or 'text'",
			})
		}

		if len(valErrors) > 0 {
			writeJSONResponse(w, http.StatusUnprocessableEntity, ValidationErrorResponse{Errors: valErrors})
			return
		}

		// создать пост
		post := model.NewPost(
			req.Title,
			model.Category(req.Category),
			req.Type,
			req.URL,
			req.Text,
			user,
		)

		// добавить пост
		postStore.AddPost(post)

		// вернуть пост
		writeJSONResponse(w, http.StatusCreated, post)
	}
}
