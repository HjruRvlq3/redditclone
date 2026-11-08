package server

import (
	"cmp"
	"net/http"
	"slices"

	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/model"
	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/store"
)

// getPostUser получение всех постов конкретного пользователя
func getPostUser(userStore *store.UserStore, postStore *store.PostStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// читать JSON
		username := r.PathValue("USER_LOGIN")
		if username == "" {
			writeJSONError(w, http.StatusBadRequest, "invalid user id")
			return
		}

		// проверить, есть ли такой юзер
		user, err := userStore.GetByUsername(username)
		if err != nil {
			if err == store.ErrUserNotFound {
				writeJSONError(w, http.StatusNotFound, store.ErrUserNotFound.Error())
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "server error")
			return
		}

		// получить посты
		postList := postStore.GetByUser(user.Id)

		slices.SortFunc(postList, func(a, b *model.Post) int {
			return cmp.Compare(b.Created, a.Created)
		})

		// отдать посты
		writeJSONResponse(w, http.StatusOK, postList)
	}
}
