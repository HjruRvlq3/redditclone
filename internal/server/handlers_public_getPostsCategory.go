package server

import (
	"cmp"
	"net/http"
	"slices"

	"redditclone/internal/model"
	"redditclone/internal/store"
)

// getPostsCategory список постов конкретной категории
func getPostsCategory(postStore *store.PostStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// получить категорию
		categoryRequest := r.PathValue("CATEGORY_NAME")
		if categoryRequest == "" {
			writeJSONError(w, http.StatusBadRequest, "category is required")
			return
		}
		category := model.Category(categoryRequest)

		// сформировать массив
		postList := postStore.GetCategory(category)
		slices.SortFunc(postList, func(a, b *model.Post) int {
			return cmp.Compare(b.Score, a.Score)
		})

		// отдать массив
		writeJSONResponse(w, http.StatusOK, postList)
	}
}
