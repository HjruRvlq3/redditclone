package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"redditclone/internal/jwtconfig"
	"redditclone/internal/model"
	"redditclone/internal/store"
)

const securityTestSecret = "security-handlers-test-secret"

// newSecurityTestMux собирает mux
func newSecurityTestMux() (*http.ServeMux, *store.PostStore, *jwtconfig.Manager) {
	postStore := store.NewPostStore()
	jwtManager := jwtconfig.NewManager([]byte(securityTestSecret))
	mux := http.NewServeMux()
	registerSecurityHandlers(mux, postStore, jwtManager)
	return mux, postStore, jwtManager
}

// securityToken генерирует валидный JWT
func securityToken(t *testing.T, m *jwtconfig.Manager, u *model.User) string {
	t.Helper()
	token, err := m.GenerateToken(u)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return token
}

// doSecurityReq выполняет запрос к mux
func doSecurityReq(mux http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func decodePost(t *testing.T, body []byte) *model.Post {
	t.Helper()
	var p model.Post
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("decode post: %v, body: %s", err, string(body))
	}
	return &p
}

func newAdminUser(username string) *model.User {
	u := model.NewUser(username, "hash")
	u.Admin = true
	return u
}

func addTestPost(t *testing.T, ps *store.PostStore, author *model.User, title string) *model.Post {
	t.Helper()
	p := model.NewPost(title, "news", "text", "", "some text here", author)
	ps.AddPost(p)
	return p
}

func addTestComment(t *testing.T, ps *store.PostStore, postID string, author *model.User, body string) *model.Comment {
	t.Helper()
	c := model.NewComment(author, body)
	if _, err := ps.PostComment(postID, c); err != nil {
		t.Fatalf("add comment: %v", err)
	}
	return c
}

func TestPostPosts(t *testing.T) {
	const validTextBody = `{"title":"Hello","category":"news","type":"text","text":"some text"}`

	t.Run("Without token returns 401", func(t *testing.T) {
		mux, _, _ := newSecurityTestMux()
		rr := doSecurityReq(mux, http.MethodPost, "/api/posts", "", validTextBody)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("With invalid token returns 401", func(t *testing.T) {
		mux, _, _ := newSecurityTestMux()
		rr := doSecurityReq(mux, http.MethodPost, "/api/posts", "not-a-jwt", validTextBody)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("Invalid JSON returns 400", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		rr := doSecurityReq(mux, http.MethodPost, "/api/posts", token, `{"title":`)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("Validation errors return 422", func(t *testing.T) {
		cases := []struct {
			name  string
			body  string
			param string
		}{
			{"empty title", `{"title":"","category":"news","type":"text","text":"some text"}`, "title"},
			{"title too long", `{"title":"` + strings.Repeat("a", 101) + `","category":"news","type":"text","text":"some text"}`, "title"},
			{"title with trailing space", `{"title":"hi ","category":"news","type":"text","text":"some text"}`, "title"},
			{"missing category", `{"title":"hi","category":"","type":"text","text":"some text"}`, "category"},
			{"missing type", `{"title":"hi","category":"news","type":"","text":"some text"}`, "type"},
			{"invalid type", `{"title":"hi","category":"news","type":"video","text":"some text"}`, "type"},
			{"missing url for link", `{"title":"hi","category":"news","type":"link","url":""}`, "url"},
			{"invalid url", `{"title":"hi","category":"news","type":"link","url":"example.com"}`, "url"},
			{"text too short", `{"title":"hi","category":"news","type":"text","text":"abc"}`, "text"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				mux, _, m := newSecurityTestMux()
				token := securityToken(t, m, model.NewUser("alice", "hash"))
				rr := doSecurityReq(mux, http.MethodPost, "/api/posts", token, tc.body)
				if rr.Code != http.StatusUnprocessableEntity {
					t.Fatalf("expected 422, got %d, body: %s", rr.Code, rr.Body.String())
				}
				var resp ValidationErrorResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode validation error: %v", err)
				}
				found := false
				for _, e := range resp.Errors {
					if e.Param == tc.param {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error for param %q, got: %+v", tc.param, resp.Errors)
				}
			})
		}
	})

	t.Run("Creates text post with author from JWT", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		user := model.NewUser("alice", "hash")
		token := securityToken(t, m, user)

		// Пытаемся подменить автора через тело запроса — хендлер должен игнорировать.
		body := `{"title":"Hello","category":"news","type":"text","text":"some text","author":{"id":"fake","username":"fake"}}`
		rr := doSecurityReq(mux, http.MethodPost, "/api/posts", token, body)

		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d, body: %s", rr.Code, rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected JSON content type, got %q", ct)
		}

		post := decodePost(t, rr.Body.Bytes())
		if post.Id == "" {
			t.Error("expected non-empty id")
		}
		if post.Title != "Hello" || post.Category != "news" || post.Type != "text" || post.Text != "some text" {
			t.Errorf("unexpected post fields: %+v", post)
		}
		if post.Author == nil || post.Author.Id != user.Id || post.Author.Username != "alice" {
			t.Errorf("author = %+v, want id=%q username=alice", post.Author, user.Id)
		}
		if post.Score != 1 || post.UpvotePercentage != 100 || post.Views != 0 {
			t.Errorf("unexpected initial counters: score=%d upvotePct=%d views=%d",
				post.Score, post.UpvotePercentage, post.Views)
		}
		if len(post.Votes) != 1 || post.Votes[0].User != user.Id || post.Votes[0].Vote != 1 {
			t.Errorf("votes = %+v, want [{user: %q, vote: 1}]", post.Votes, user.Id)
		}
		if post.Comments == nil || len(post.Comments) != 0 {
			t.Errorf("comments = %+v, want []", post.Comments)
		}

		// Пост реально сохранился в хранилище.
		if _, err := postStore.GetById(post.Id); err != nil {
			t.Fatalf("post not found in store: %v", err)
		}
	})

	t.Run("Creates link post", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("bob", "hash"))
		body := `{"title":"Go","category":"programming","type":"link","url":"https://go.dev/"}`
		rr := doSecurityReq(mux, http.MethodPost, "/api/posts", token, body)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d, body: %s", rr.Code, rr.Body.String())
		}
		post := decodePost(t, rr.Body.Bytes())
		if post.Type != "link" || post.URL != "https://go.dev/" || post.Text != "" {
			t.Errorf("unexpected link post: %+v", post)
		}
	})

	t.Run("Trailing slash path also works", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		rr := doSecurityReq(mux, http.MethodPost, "/api/posts/", token, validTextBody)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d, body: %s", rr.Code, rr.Body.String())
		}
	})
}

func TestPostComment(t *testing.T) {
	t.Run("Without token returns 401", func(t *testing.T) {
		mux, postStore, _ := newSecurityTestMux()
		p := addTestPost(t, postStore, model.NewUser("alice", "hash"), "post")
		rr := doSecurityReq(mux, http.MethodPost, "/api/post/"+p.Id, "", `{"comment":"hi"}`)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("Invalid post id returns 400", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		rr := doSecurityReq(mux, http.MethodPost, "/api/post/not-a-uuid", token, `{"comment":"hi"}`)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("Invalid JSON returns 400", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		p := addTestPost(t, postStore, model.NewUser("alice", "hash"), "post")
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		rr := doSecurityReq(mux, http.MethodPost, "/api/post/"+p.Id, token, `{"comment":`)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("Post not found returns 404", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		// Валидный UUID, которого нет в хранилище.
		rr := doSecurityReq(mux, http.MethodPost, "/api/post/00000000-0000-4000-8000-000000000000", token, `{"comment":"hi"}`)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("Empty comment returns 422", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		p := addTestPost(t, postStore, model.NewUser("alice", "hash"), "post")
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		rr := doSecurityReq(mux, http.MethodPost, "/api/post/"+p.Id, token, `{"comment":""}`)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d, body: %s", rr.Code, rr.Body.String())
		}
		var resp ValidationErrorResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp.Errors) == 0 || resp.Errors[0].Param != "comment" {
			t.Errorf("expected error on param comment, got %+v", resp.Errors)
		}
	})

	t.Run("Comment too long returns 422", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		p := addTestPost(t, postStore, model.NewUser("alice", "hash"), "post")
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		body := `{"comment":"` + strings.Repeat("x", 2001) + `"}`
		rr := doSecurityReq(mux, http.MethodPost, "/api/post/"+p.Id, token, body)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d", rr.Code)
		}
	})

	t.Run("Creates comment and returns full post", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post")

		bob := model.NewUser("bob", "hash")
		token := securityToken(t, m, bob)

		rr := doSecurityReq(mux, http.MethodPost, "/api/post/"+p.Id, token, `{"comment":"Thanks for sharing!"}`)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d, body: %s", rr.Code, rr.Body.String())
		}

		post := decodePost(t, rr.Body.Bytes())
		if post.Id != p.Id {
			t.Errorf("id = %q, want %q", post.Id, p.Id)
		}
		if len(post.Comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(post.Comments))
		}
		c := post.Comments[0]
		if c.Body != "Thanks for sharing!" {
			t.Errorf("comment body = %q", c.Body)
		}
		if c.Author == nil || c.Author.Username != "bob" {
			t.Errorf("comment author = %+v, want bob", c.Author)
		}
	})
}

func TestGetUpvote(t *testing.T) {
	t.Run("Without token returns 401", func(t *testing.T) {
		mux, postStore, _ := newSecurityTestMux()
		p := addTestPost(t, postStore, model.NewUser("alice", "hash"), "post")
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/"+p.Id+"/upvote", "", "")
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("Invalid post id returns 400", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/not-a-uuid/upvote", token, "")
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("Post not found returns 404", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/00000000-0000-4000-8000-000000000000/upvote", token, "")
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rr.Code)
		}
	})

	t.Run("Author upvoting own post is idempotent", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post") // уже имеет голос автора: score=1
		token := securityToken(t, m, author)

		rr := doSecurityReq(mux, http.MethodGet, "/api/post/"+p.Id+"/upvote", token, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
		}
		post := decodePost(t, rr.Body.Bytes())
		if post.Score != 1 || post.UpvotePercentage != 100 {
			t.Errorf("score = %d, upvotePct = %d, want 1 / 100", post.Score, post.UpvotePercentage)
		}
		if len(post.Votes) != 1 {
			t.Errorf("expected 1 vote, got %d", len(post.Votes))
		}
	})

	t.Run("Another user adds a new upvote", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post")

		bob := model.NewUser("bob", "hash")
		token := securityToken(t, m, bob)
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/"+p.Id+"/upvote", token, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
		}

		post := decodePost(t, rr.Body.Bytes())
		if post.Score != 2 {
			t.Errorf("score = %d, want 2", post.Score)
		}
		if post.UpvotePercentage != 100 {
			t.Errorf("upvotePercentage = %d, want 100", post.UpvotePercentage)
		}
		if len(post.Votes) != 2 {
			t.Errorf("expected 2 votes, got %d", len(post.Votes))
		}
	})

	t.Run("Repeat upvote by same user does not add a vote", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post")

		bob := model.NewUser("bob", "hash")
		token := securityToken(t, m, bob)

		for i := 0; i < 2; i++ {
			rr := doSecurityReq(mux, http.MethodGet, "/api/post/"+p.Id+"/upvote", token, "")
			if rr.Code != http.StatusOK {
				t.Fatalf("iter %d: expected 200, got %d", i, rr.Code)
			}
		}

		stored, err := postStore.GetById(p.Id)
		if err != nil {
			t.Fatalf("get post: %v", err)
		}
		if stored.Score != 2 {
			t.Errorf("score = %d, want 2", stored.Score)
		}
		if len(stored.Votes) != 2 {
			t.Errorf("expected 2 votes, got %d", len(stored.Votes))
		}
	})

	t.Run("Upvote replaces a previous downvote", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post")

		bob := model.NewUser("bob", "hash")
		token := securityToken(t, m, bob)

		// Сначала даунвоут, потом upvote.
		if rr := doSecurityReq(mux, http.MethodGet, "/api/post/"+p.Id+"/downvote", token, ""); rr.Code != http.StatusOK {
			t.Fatalf("downvote: expected 200, got %d", rr.Code)
		}
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/"+p.Id+"/upvote", token, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("upvote: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
		}

		post := decodePost(t, rr.Body.Bytes())
		if post.Score != 2 {
			t.Errorf("score = %d, want 2", post.Score)
		}
		if len(post.Votes) != 2 {
			t.Errorf("expected 2 votes (author+1, bob+1), got %d", len(post.Votes))
		}
		for _, v := range post.Votes {
			if v.Vote != 1 {
				t.Errorf("expected all votes = 1, got %+v", v)
			}
		}
	})
}

func TestGetDownvote(t *testing.T) {
	t.Run("Without token returns 401", func(t *testing.T) {
		mux, postStore, _ := newSecurityTestMux()
		p := addTestPost(t, postStore, model.NewUser("alice", "hash"), "post")
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/"+p.Id+"/downvote", "", "")
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("Invalid post id returns 400", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/not-a-uuid/downvote", token, "")
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("Post not found returns 404", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/00000000-0000-4000-8000-000000000000/downvote", token, "")
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rr.Code)
		}
	})

	t.Run("Author downvotes own post", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post") // изначально {author: +1}
		token := securityToken(t, m, author)

		rr := doSecurityReq(mux, http.MethodGet, "/api/post/"+p.Id+"/downvote", token, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
		}
		post := decodePost(t, rr.Body.Bytes())
		if post.Score != -1 {
			t.Errorf("score = %d, want -1", post.Score)
		}
		if post.UpvotePercentage != 0 {
			t.Errorf("upvotePercentage = %d, want 0", post.UpvotePercentage)
		}
		if len(post.Votes) != 1 || post.Votes[0].Vote != -1 {
			t.Errorf("votes = %+v, want single -1", post.Votes)
		}
	})

	t.Run("Mixed votes give 50 percent", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post") // votes: [alice:+1]

		bob := model.NewUser("bob", "hash")
		token := securityToken(t, m, bob)
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/"+p.Id+"/downvote", token, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		post := decodePost(t, rr.Body.Bytes())
		if post.Score != 0 {
			t.Errorf("score = %d, want 0", post.Score)
		}
		if post.UpvotePercentage != 50 {
			t.Errorf("upvotePercentage = %d, want 50", post.UpvotePercentage)
		}
	})
}

func TestGetUnvote(t *testing.T) {
	t.Run("Without token returns 401", func(t *testing.T) {
		mux, postStore, _ := newSecurityTestMux()
		p := addTestPost(t, postStore, model.NewUser("alice", "hash"), "post")
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/"+p.Id+"/unvote", "", "")
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("Invalid post id returns 400", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/not-a-uuid/unvote", token, "")
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("Post not found returns 404", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/00000000-0000-4000-8000-000000000000/unvote", token, "")
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rr.Code)
		}
	})

	t.Run("Removes existing vote and encodes votes as []", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post") // votes: [alice:+1]

		token := securityToken(t, m, author)
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/"+p.Id+"/unvote", token, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
		}

		// Пустой votes должен кодироваться как [], а не null.
		if body := strings.TrimSpace(rr.Body.String()); strings.Contains(body, `"votes":null`) {
			t.Errorf("votes must be [] not null, body: %s", body)
		}

		post := decodePost(t, rr.Body.Bytes())
		if post.Score != 0 {
			t.Errorf("score = %d, want 0", post.Score)
		}
		if post.UpvotePercentage != 0 {
			t.Errorf("upvotePercentage = %d, want 0", post.UpvotePercentage)
		}
		if post.Votes == nil {
			t.Fatal("votes is nil, want empty slice")
		}
		if len(post.Votes) != 0 {
			t.Errorf("expected 0 votes, got %d", len(post.Votes))
		}
	})

	t.Run("Unvote without previous vote is a no-op", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post") // votes: [alice:+1]

		// bob не голосовал — unvote не должен ничего менять.
		bob := model.NewUser("bob", "hash")
		token := securityToken(t, m, bob)
		rr := doSecurityReq(mux, http.MethodGet, "/api/post/"+p.Id+"/unvote", token, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}

		post := decodePost(t, rr.Body.Bytes())
		if post.Score != 1 {
			t.Errorf("score = %d, want 1 (unchanged)", post.Score)
		}
		if len(post.Votes) != 1 {
			t.Errorf("expected 1 vote, got %d", len(post.Votes))
		}
		if post.Votes[0].User != author.Id || post.Votes[0].Vote != 1 {
			t.Errorf("alice's vote changed: %+v", post.Votes[0])
		}
	})
}

func TestDeletePost(t *testing.T) {
	t.Run("Without token returns 401", func(t *testing.T) {
		mux, postStore, _ := newSecurityTestMux()
		p := addTestPost(t, postStore, model.NewUser("alice", "hash"), "post")
		rr := doSecurityReq(mux, http.MethodDelete, "/api/post/"+p.Id, "", "")
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("Invalid post id returns 400", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		rr := doSecurityReq(mux, http.MethodDelete, "/api/post/not-a-uuid", token, "")
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("Post not found returns 404", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))
		rr := doSecurityReq(mux, http.MethodDelete, "/api/post/00000000-0000-4000-8000-000000000000", token, "")
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rr.Code)
		}
	})

	t.Run("Non-author non-admin gets 401", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post")

		bob := model.NewUser("bob", "hash")
		token := securityToken(t, m, bob)
		rr := doSecurityReq(mux, http.MethodDelete, "/api/post/"+p.Id, token, "")
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("Author deletes successfully", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post")

		token := securityToken(t, m, author)
		rr := doSecurityReq(mux, http.MethodDelete, "/api/post/"+p.Id, token, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected JSON content type, got %q", ct)
		}

		var resp struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Message != "success" {
			t.Errorf("message = %q, want success", resp.Message)
		}

		// Пост исчез из хранилища.
		if _, err := postStore.GetById(p.Id); err == nil {
			t.Error("post still exists in store")
		} else if err != store.ErrPostNotFound {
			t.Errorf("expected ErrPostNotFound, got %v", err)
		}
	})

	t.Run("Admin deletes any post", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post")

		admin := newAdminUser("admin")
		token := securityToken(t, m, admin)
		rr := doSecurityReq(mux, http.MethodDelete, "/api/post/"+p.Id, token, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
		}
	})
}

func TestDeleteComment(t *testing.T) {
	t.Run("Without token returns 401", func(t *testing.T) {
		mux, postStore, _ := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post")
		c := addTestComment(t, postStore, p.Id, author, "hello")

		rr := doSecurityReq(mux, http.MethodDelete, "/api/post/"+p.Id+"/"+c.Id, "", "")
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("With invalid token returns 401", func(t *testing.T) {
		mux, postStore, _ := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post")
		c := addTestComment(t, postStore, p.Id, author, "hello")

		rr := doSecurityReq(mux, http.MethodDelete, "/api/post/"+p.Id+"/"+c.Id, "not-a-jwt", "")
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("Invalid post id returns 400", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))

		rr := doSecurityReq(mux, http.MethodDelete, "/api/post/not-a-uuid/some-comment", token, "")
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d, body: %s", rr.Code, rr.Body.String())
		}
		var resp struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Message != "invalid post id" {
			t.Errorf("message = %q, want %q", resp.Message, "invalid post id")
		}
	})

	t.Run("Post not found returns 404", func(t *testing.T) {
		mux, _, m := newSecurityTestMux()
		token := securityToken(t, m, model.NewUser("alice", "hash"))

		rr := doSecurityReq(mux, http.MethodDelete,
			"/api/post/00000000-0000-4000-8000-000000000000/00000000-0000-4000-8000-000000000001",
			token, "")
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d, body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("Comment not found returns 404", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		author := model.NewUser("alice", "hash")
		p := addTestPost(t, postStore, author, "post")

		token := securityToken(t, m, author)
		rr := doSecurityReq(mux, http.MethodDelete,
			"/api/post/"+p.Id+"/00000000-0000-4000-8000-000000000001",
			token, "")
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d, body: %s", rr.Code, rr.Body.String())
		}
		var resp struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Message != "comment not found" {
			t.Errorf("message = %q, want %q", resp.Message, "comment not found")
		}
	})

	t.Run("Stranger cannot delete comment", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		postAuthor := model.NewUser("alice", "hash")
		commentAuthor := model.NewUser("bob", "hash")
		p := addTestPost(t, postStore, postAuthor, "post")
		c := addTestComment(t, postStore, p.Id, commentAuthor, "hello")

		charlie := model.NewUser("charlie", "hash")
		token := securityToken(t, m, charlie)

		rr := doSecurityReq(mux, http.MethodDelete, "/api/post/"+p.Id+"/"+c.Id, token, "")
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d, body: %s", rr.Code, rr.Body.String())
		}

		// Комментарий остался в сторе.
		stored, err := postStore.GetById(p.Id)
		if err != nil {
			t.Fatalf("get post: %v", err)
		}
		if len(stored.Comments) != 1 {
			t.Errorf("expected comment to remain, got %d comments", len(stored.Comments))
		}
	})

	t.Run("Comment author can delete and gets full post", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		postAuthor := model.NewUser("alice", "hash")
		commentAuthor := model.NewUser("bob", "hash")
		p := addTestPost(t, postStore, postAuthor, "post")
		c := addTestComment(t, postStore, p.Id, commentAuthor, "hello")

		token := securityToken(t, m, commentAuthor)
		rr := doSecurityReq(mux, http.MethodDelete, "/api/post/"+p.Id+"/"+c.Id, token, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected JSON content type, got %q", ct)
		}

		post := decodePost(t, rr.Body.Bytes())
		if post.Id != p.Id {
			t.Errorf("id = %q, want %q", post.Id, p.Id)
		}
		if len(post.Comments) != 0 {
			t.Errorf("expected 0 comments, got %d", len(post.Comments))
		}

		// Проверяем, что комментарий действительно удалён из хранилища.
		stored, err := postStore.GetById(p.Id)
		if err != nil {
			t.Fatalf("get post: %v", err)
		}
		if len(stored.Comments) != 0 {
			t.Errorf("comment still exists in store: %+v", stored.Comments)
		}
	})

	t.Run("Post author can delete any comment", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		postAuthor := model.NewUser("alice", "hash")
		commentAuthor := model.NewUser("bob", "hash")
		p := addTestPost(t, postStore, postAuthor, "post")
		c := addTestComment(t, postStore, p.Id, commentAuthor, "hello")

		token := securityToken(t, m, postAuthor)
		rr := doSecurityReq(mux, http.MethodDelete, "/api/post/"+p.Id+"/"+c.Id, token, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
		}

		post := decodePost(t, rr.Body.Bytes())
		if len(post.Comments) != 0 {
			t.Errorf("expected 0 comments, got %d", len(post.Comments))
		}
	})

	t.Run("Admin can delete any comment", func(t *testing.T) {
		mux, postStore, m := newSecurityTestMux()
		postAuthor := model.NewUser("alice", "hash")
		commentAuthor := model.NewUser("bob", "hash")
		p := addTestPost(t, postStore, postAuthor, "post")
		c := addTestComment(t, postStore, p.Id, commentAuthor, "hello")

		admin := newAdminUser("admin")
		token := securityToken(t, m, admin)
		rr := doSecurityReq(mux, http.MethodDelete, "/api/post/"+p.Id+"/"+c.Id, token, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", rr.Code, rr.Body.String())
		}

		post := decodePost(t, rr.Body.Bytes())
		if len(post.Comments) != 0 {
			t.Errorf("expected 0 comments, got %d", len(post.Comments))
		}
	})
}
