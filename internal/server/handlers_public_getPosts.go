package server

import (
	"cmp"
	"net/http"
	"slices"

	"redditclone/internal/model"
	"redditclone/internal/store"
)

// getPosts список всех постов
func getPosts(postStore *store.PostStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// сформировать массив
		postList := postStore.GetAll()
		slices.SortFunc(postList, func(a, b *model.Post) int {
			return cmp.Compare(b.Score, a.Score)
		})

		// отдать массив
		writeJSONResponse(w, http.StatusOK, postList)
	}
}
