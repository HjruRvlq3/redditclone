package server

import (
	"net/http"

	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/jwtconfig"
	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/store"
)

// registerPublicHandlers ...
func registerPublicHandlers(mux *http.ServeMux, userStore *store.UserStore, postStore *store.PostStore, jwtManager *jwtconfig.Manager) {
	mux.HandleFunc("POST /api/register", postRegister(userStore, jwtManager))
	mux.HandleFunc("POST /api/login", postLogin(userStore, jwtManager))
	mux.HandleFunc("GET /api/posts", getPosts(postStore))
	mux.HandleFunc("GET /api/posts/", getPosts(postStore))
	mux.HandleFunc("GET /api/posts/{CATEGORY_NAME}", getPostsCategory(postStore))
	mux.HandleFunc("GET /api/post/{POST_ID}", getPostId(postStore))
	mux.HandleFunc("GET /api/user/{USER_LOGIN}", getPostUser(userStore, postStore))
}
