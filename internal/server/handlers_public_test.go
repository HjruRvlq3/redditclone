package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"redditclone/internal/jwtconfig"
	"redditclone/internal/model"
	"redditclone/internal/store"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func TestPostRegister(t *testing.T) {
	// Табличные тесты
	tests := []struct {
		name           string
		requestBody    string
		setupUser      func(*store.UserStore) // Функция для предварительного создания юзера
		expectedStatus int
	}{
		{
			name:           "Success",
			requestBody:    `{"username": "valid_user", "password": "password123"}`,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "Invalid JSON",
			requestBody:    `{"username": "valid_user", "password": `, // Сломаный JSON
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Username too short",
			requestBody:    `{"username": "ab", "password": "password123"}`,
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "Username too long",
			requestBody:    `{"username": "this_is_a_very_long_username_which_is_definitely_more_than_32_chars", "password": "password123"}`,
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "Username with invalid characters",
			requestBody:    `{"username": "invalid user!", "password": "password123"}`,
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "Password too short",
			requestBody:    `{"username": "valid_user", "password": "123"}`,
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "Multiple validation errors",
			requestBody:    `{"username": "a", "password": "1"}`,
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:        "User already exists",
			requestBody: `{"username": "existing_user", "password": "password123"}`,
			setupUser: func(us *store.UserStore) {
				// Заранее создаем юзера с таким же именем
				_ = us.AddUser(model.NewUser("existing_user", "hash"))
			},
			expectedStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Создаем ЧИСТОЕ окружение для каждого теста
			userStore := store.NewUserStore()
			jwtManager := jwtconfig.NewManager([]byte("test-secret-key"))
			handler := postRegister(userStore, jwtManager)

			// 2. Выполняем предварительную настройку, если она нужна (для теста дубликата)
			if tt.setupUser != nil {
				tt.setupUser(userStore)
			}

			// 3. Создаем фейковый запрос
			req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			// 4. Вызываем хендлер
			handler.ServeHTTP(rr, req)

			// 5. Проверяем статус-код
			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}

			// 6. Дополнительные проверки для успешного ответа
			if tt.expectedStatus == http.StatusCreated {
				var resp map[string]string
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if resp["token"] == "" {
					t.Errorf("expected non-empty token in response")
				}
			}

			// 7. Проверка для ошибок валидации (422)
			if tt.expectedStatus == http.StatusUnprocessableEntity {
				var resp ValidationErrorResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal validation error response: %v", err)
				}
				if len(resp.Errors) == 0 {
					t.Errorf("expected errors array in response, got empty")
				}
			}
		})
	}
}

func TestPostLogin(t *testing.T) {
	const (
		secret        = "test-secret-key"
		validUsername = "alice"
		validPassword = "password123"
	)

	createUser := func(us *store.UserStore) *model.User {
		hash, err := bcrypt.GenerateFromPassword([]byte(validPassword), bcrypt.MinCost)
		if err != nil {
			t.Fatalf("bcrypt: %v", err)
		}
		u := model.NewUser(validUsername, string(hash))
		if err := us.AddUser(u); err != nil {
			t.Fatalf("add user: %v", err)
		}
		return u
	}

	tests := []struct {
		name            string
		body            string
		setup           func(*store.UserStore) *model.User
		expectedStatus  int
		expectedMessage string
		expectToken     bool
	}{
		{
			name:           "Success",
			body:           `{"username":"alice","password":"password123"}`,
			setup:          createUser,
			expectedStatus: http.StatusOK,
			expectToken:    true,
		},
		{
			name:           "Success with different case username",
			body:           `{"username":"ALICE","password":"password123"}`,
			setup:          createUser,
			expectedStatus: http.StatusOK,
			expectToken:    true,
		},
		{
			name:            "Invalid JSON",
			body:            `{"username": "alice", "password": `,
			expectedStatus:  http.StatusBadRequest,
			expectedMessage: "invalid json",
		},
		{
			name:            "Empty body",
			body:            ``,
			expectedStatus:  http.StatusBadRequest,
			expectedMessage: "invalid json",
		},
		{
			name:            "Unknown user",
			body:            `{"username":"nobody","password":"password123"}`,
			expectedStatus:  http.StatusUnauthorized,
			expectedMessage: "user not found",
		},
		{
			name:            "Wrong password",
			body:            `{"username":"alice","password":"wrong-password"}`,
			setup:           createUser,
			expectedStatus:  http.StatusUnauthorized,
			expectedMessage: "invalid password",
		},
		{
			name:            "Empty password",
			body:            `{"username":"alice","password":""}`,
			setup:           createUser,
			expectedStatus:  http.StatusUnauthorized,
			expectedMessage: "invalid password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userStore := store.NewUserStore()
			jwtManager := jwtconfig.NewManager([]byte(secret))
			handler := postLogin(userStore, jwtManager)

			var expectedUser *model.User
			if tt.setup != nil {
				expectedUser = tt.setup(userStore)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d, body: %s",
					tt.expectedStatus, rr.Code, rr.Body.String())
			}

			if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("expected JSON content type, got %q", ct)
			}

			// Ветки с ошибкой: проверяем {message: ...}.
			if tt.expectedMessage != "" {
				var resp struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("unmarshal error response: %v", err)
				}
				if resp.Message != tt.expectedMessage {
					t.Errorf("expected message %q, got %q", tt.expectedMessage, resp.Message)
				}
				return
			}

			if !tt.expectToken {
				return
			}

			// Успех: проверяем {token} и содержимое JWT.
			var resp struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal success response: %v", err)
			}
			if resp.Token == "" {
				t.Fatal("expected non-empty token")
			}

			claims := &jwtconfig.Claims{}
			parsed, err := jwt.ParseWithClaims(resp.Token, claims, func(tk *jwt.Token) (interface{}, error) {
				if tk.Method.Alg() != "HS256" {
					t.Fatalf("unexpected signing method: %s", tk.Method.Alg())
				}
				return []byte(secret), nil
			})
			if err != nil || !parsed.Valid {
				t.Fatalf("invalid token: %v", err)
			}

			if claims.User == nil {
				t.Fatal("claims.user is nil")
			}
			if claims.User.Id != expectedUser.Id {
				t.Errorf("claims.user.id = %q, want %q", claims.User.Id, expectedUser.Id)
			}
			if claims.User.Username != validUsername {
				t.Errorf("claims.user.username = %q, want %q", claims.User.Username, validUsername)
			}

			if claims.IssuedAt == nil || claims.ExpiresAt == nil {
				t.Fatal("iat/exp must be set")
			}
			if !claims.ExpiresAt.After(claims.IssuedAt.Time) {
				t.Error("exp must be after iat")
			}
			if lifetime := claims.ExpiresAt.Sub(claims.IssuedAt.Time); lifetime != 7*24*time.Hour {
				t.Errorf("token lifetime = %v, want 7 days", lifetime)
			}

			// В payload не должно быть пароля или его хеша.
			claimsJSON, err := json.Marshal(claims)
			if err != nil {
				t.Fatalf("marshal claims: %v", err)
			}
			if strings.Contains(strings.ToLower(string(claimsJSON)), "password") {
				t.Errorf("token payload must not contain password: %s", claimsJSON)
			}
		})
	}
}

func TestGetPosts(t *testing.T) {
	t.Run("Empty store returns empty array", func(t *testing.T) {
		postStore := store.NewPostStore()
		handler := getPosts(postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rr.Code, rr.Body.String())
		}

		if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected JSON content type, got %q", ct)
		}

		// Пустая коллекция должна кодироваться как [], а не null.
		if body := strings.TrimSpace(rr.Body.String()); body != "[]" {
			t.Errorf("expected empty array [], got %q", body)
		}

		var posts []*model.Post
		if err := json.Unmarshal(rr.Body.Bytes(), &posts); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if posts == nil {
			t.Fatal("expected non-nil slice, got null")
		}
		if len(posts) != 0 {
			t.Errorf("expected 0 posts, got %d", len(posts))
		}
	})

	t.Run("Returns all posts sorted by score descending", func(t *testing.T) {
		postStore := store.NewPostStore()

		author := model.NewUser("alice", "hash")

		// Создаём посты с разными score. NewPost даёт score = 1,
		// поэтому явно задаём нужные значения после добавления.
		low := model.NewPost("low", "news", "text", "", "low post text", author)
		mid := model.NewPost("mid", "news", "text", "", "mid post text", author)
		high := model.NewPost("high", "news", "text", "", "high post text", author)
		low.Score = -3
		mid.Score = 5
		high.Score = 10

		postStore.AddPost(low)
		postStore.AddPost(mid)
		postStore.AddPost(high)

		handler := getPosts(postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rr.Code, rr.Body.String())
		}

		if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected JSON content type, got %q", ct)
		}

		var posts []*model.Post
		if err := json.Unmarshal(rr.Body.Bytes(), &posts); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if len(posts) != 3 {
			t.Fatalf("expected 3 posts, got %d", len(posts))
		}

		// Порядок должен быть по убыванию score.
		wantOrder := []string{"high", "mid", "low"}
		for i, want := range wantOrder {
			if posts[i].Title != want {
				t.Errorf("position %d: expected %q, got %q (score=%d)",
					i, want, posts[i].Title, posts[i].Score)
			}
		}

		// Проверяем, что score действительно по убыванию.
		for i := 1; i < len(posts); i++ {
			if posts[i-1].Score < posts[i].Score {
				t.Errorf("posts not sorted by score desc: [%d]=%d < [%d]=%d",
					i-1, posts[i-1].Score, i, posts[i].Score)
			}
		}
	})

	t.Run("Posts with equal score are all returned", func(t *testing.T) {
		postStore := store.NewPostStore()

		author := model.NewUser("bob", "hash")
		a := model.NewPost("a", "news", "text", "", "aaaa", author)
		b := model.NewPost("b", "news", "text", "", "bbbb", author)
		a.Score = 2
		b.Score = 2

		postStore.AddPost(a)
		postStore.AddPost(b)

		handler := getPosts(postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}

		var posts []*model.Post
		if err := json.Unmarshal(rr.Body.Bytes(), &posts); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if len(posts) != 2 {
			t.Fatalf("expected 2 posts, got %d", len(posts))
		}
		for _, p := range posts {
			if p.Score != 2 {
				t.Errorf("expected score 2, got %d", p.Score)
			}
		}
	})
}

func TestGetPostsCategory(t *testing.T) {
	t.Run("Empty category returns empty array", func(t *testing.T) {
		postStore := store.NewPostStore()
		handler := getPostsCategory(postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/posts/news", nil)
		req.SetPathValue("CATEGORY_NAME", "news")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rr.Code, rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected JSON content type, got %q", ct)
		}

		// Пустая коллекция должна кодироваться как [], а не null.
		if body := strings.TrimSpace(rr.Body.String()); body != "[]" {
			t.Errorf("expected empty array [], got %q", body)
		}

		var posts []*model.Post
		if err := json.Unmarshal(rr.Body.Bytes(), &posts); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if posts == nil {
			t.Fatal("expected non-nil slice, got null")
		}
		if len(posts) != 0 {
			t.Errorf("expected 0 posts, got %d", len(posts))
		}
	})

	t.Run("Missing category returns 400", func(t *testing.T) {
		postStore := store.NewPostStore()
		handler := getPostsCategory(postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/posts/", nil)
		// CATEGORY_NAME намеренно не задаём
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s", rr.Code, rr.Body.String())
		}

		var resp struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error response: %v", err)
		}
		if resp.Message != "category is required" {
			t.Errorf("expected message %q, got %q", "category is required", resp.Message)
		}
	})

	t.Run("Filters by category and sorts by score desc", func(t *testing.T) {
		postStore := store.NewPostStore()
		author := model.NewUser("alice", "hash")

		// Два поста в programming с разным score.
		low := model.NewPost("low prog", "programming", "text", "", "low text", author)
		high := model.NewPost("high prog", "programming", "text", "", "high text", author)
		low.Score = 1
		high.Score = 7

		// Посты в других категориях не должны попасть в ответ.
		other1 := model.NewPost("news post", "news", "text", "", "news text", author)
		other2 := model.NewPost("funny post", "funny", "text", "", "funny text", author)
		other1.Score = 100
		other2.Score = 100

		postStore.AddPost(low)
		postStore.AddPost(high)
		postStore.AddPost(other1)
		postStore.AddPost(other2)

		handler := getPostsCategory(postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/posts/programming", nil)
		req.SetPathValue("CATEGORY_NAME", "programming")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rr.Code, rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected JSON content type, got %q", ct)
		}

		var posts []*model.Post
		if err := json.Unmarshal(rr.Body.Bytes(), &posts); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if len(posts) != 2 {
			t.Fatalf("expected 2 posts, got %d", len(posts))
		}

		for i, p := range posts {
			if p.Category != "programming" {
				t.Errorf("position %d: expected category programming, got %q", i, p.Category)
			}
		}

		// Сортировка по убыванию score.
		if posts[0].Title != "high prog" || posts[1].Title != "low prog" {
			t.Errorf("expected order [high prog, low prog], got [%s, %s]",
				posts[0].Title, posts[1].Title)
		}
		if posts[0].Score < posts[1].Score {
			t.Errorf("posts not sorted by score desc: %d < %d", posts[0].Score, posts[1].Score)
		}
	})

	t.Run("Unknown category returns empty array", func(t *testing.T) {
		postStore := store.NewPostStore()
		author := model.NewUser("bob", "hash")
		postStore.AddPost(model.NewPost("news post", "news", "text", "", "text", author))

		handler := getPostsCategory(postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/posts/unknown", nil)
		req.SetPathValue("CATEGORY_NAME", "unknown")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
		if body := strings.TrimSpace(rr.Body.String()); body != "[]" {
			t.Errorf("expected empty array [], got %q", body)
		}
	})
}

func TestGetPostId(t *testing.T) {
	t.Run("Missing post id returns 400", func(t *testing.T) {
		postStore := store.NewPostStore()
		handler := getPostId(postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/post/", nil)
		// POST_ID намеренно не задаём
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s", rr.Code, rr.Body.String())
		}

		var resp struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error response: %v", err)
		}
		if resp.Message != "invalid post id" {
			t.Errorf("expected message %q, got %q", "invalid post id", resp.Message)
		}
	})

	t.Run("Unknown post returns 404", func(t *testing.T) {
		postStore := store.NewPostStore()
		handler := getPostId(postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/post/missing", nil)
		req.SetPathValue("POST_ID", "missing")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d, body: %s", rr.Code, rr.Body.String())
		}

		var resp struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error response: %v", err)
		}
		if resp.Message != "post not found" {
			t.Errorf("expected message %q, got %q", "post not found", resp.Message)
		}
	})

	t.Run("Existing post is returned with full structure", func(t *testing.T) {
		postStore := store.NewPostStore()

		author := model.NewUser("alice", "hash")
		post := model.NewPost(
			"The Go programming language",
			"programming",
			"link",
			"https://go.dev/",
			"",
			author,
		)
		postStore.AddPost(post)

		handler := getPostId(postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/post/"+post.Id, nil)
		req.SetPathValue("POST_ID", post.Id)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rr.Code, rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected JSON content type, got %q", ct)
		}

		var got model.Post
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		// Проверяем, что вернулся полный объект поста.
		if got.Id != post.Id {
			t.Errorf("id = %q, want %q", got.Id, post.Id)
		}
		if got.Title != post.Title {
			t.Errorf("title = %q, want %q", got.Title, post.Title)
		}
		if got.Category != post.Category {
			t.Errorf("category = %q, want %q", got.Category, post.Category)
		}
		if got.Type != "link" {
			t.Errorf("type = %q, want %q", got.Type, "link")
		}
		if got.URL != "https://go.dev/" {
			t.Errorf("url = %q, want %q", got.URL, "https://go.dev/")
		}
		if got.Author == nil || got.Author.Username != "alice" {
			t.Errorf("author = %+v, want username alice", got.Author)
		}
		if got.Score != 1 {
			t.Errorf("score = %d, want 1", got.Score)
		}
		if got.UpvotePercentage != 100 {
			t.Errorf("upvotePercentage = %d, want 100", got.UpvotePercentage)
		}
		if got.Votes == nil {
			t.Error("votes must not be null")
		}
		if got.Comments == nil {
			t.Error("comments must not be null")
		}
		if len(got.Votes) != 1 {
			t.Errorf("expected 1 vote, got %d", len(got.Votes))
		}
		if len(got.Comments) != 0 {
			t.Errorf("expected 0 comments, got %d", len(got.Comments))
		}
		// У нового поста views = 0, после GET — 1.
		if got.Views != 1 {
			t.Errorf("views = %d, want 1", got.Views)
		}
	})

	t.Run("Each GET increments views", func(t *testing.T) {
		postStore := store.NewPostStore()

		author := model.NewUser("bob", "hash")
		post := model.NewPost("text", "news", "text", "", "some text", author)
		postStore.AddPost(post)

		handler := getPostId(postStore)

		// Трижды дёргаем GET — views должно вырасти с 0 до 3.
		for i := 1; i <= 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/post/"+post.Id, nil)
			req.SetPathValue("POST_ID", post.Id)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("iteration %d: expected 200, got %d", i, rr.Code)
			}

			var got model.Post
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatalf("iteration %d: unmarshal response: %v", i, err)
			}
			if got.Views != i {
				t.Errorf("iteration %d: views = %d, want %d", i, got.Views, i)
			}
		}
	})

	t.Run("Comments are returned in the post", func(t *testing.T) {
		postStore := store.NewPostStore()

		author := model.NewUser("alice", "hash")
		post := model.NewPost("text", "news", "text", "", "some text", author)
		postStore.AddPost(post)

		comment := model.NewComment(author, "nice post!")
		if _, err := postStore.PostComment(post.Id, comment); err != nil {
			t.Fatalf("add comment: %v", err)
		}

		handler := getPostId(postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/post/"+post.Id, nil)
		req.SetPathValue("POST_ID", post.Id)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}

		var got model.Post
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if len(got.Comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(got.Comments))
		}
		if got.Comments[0].Body != "nice post!" {
			t.Errorf("comment body = %q, want %q", got.Comments[0].Body, "nice post!")
		}
		if got.Comments[0].Author == nil || got.Comments[0].Author.Username != "alice" {
			t.Errorf("comment author = %+v, want username alice", got.Comments[0].Author)
		}
	})
}

func TestGetPostUser(t *testing.T) {
	t.Run("Missing username returns 400", func(t *testing.T) {
		userStore := store.NewUserStore()
		postStore := store.NewPostStore()
		handler := getPostUser(userStore, postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/user/", nil)
		// USER_LOGIN намеренно не задаём
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d, body: %s", rr.Code, rr.Body.String())
		}

		var resp struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error response: %v", err)
		}
		if resp.Message != "invalid user id" {
			t.Errorf("expected message %q, got %q", "invalid user id", resp.Message)
		}
	})

	t.Run("Unknown user returns 404", func(t *testing.T) {
		userStore := store.NewUserStore()
		postStore := store.NewPostStore()
		handler := getPostUser(userStore, postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/user/nobody", nil)
		req.SetPathValue("USER_LOGIN", "nobody")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d, body: %s", rr.Code, rr.Body.String())
		}

		var resp struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal error response: %v", err)
		}
		if resp.Message != "user not found" {
			t.Errorf("expected message %q, got %q", "user not found", resp.Message)
		}
	})

	t.Run("Existing user without posts returns empty array", func(t *testing.T) {
		userStore := store.NewUserStore()
		postStore := store.NewPostStore()

		alice := model.NewUser("alice", "hash")
		if err := userStore.AddUser(alice); err != nil {
			t.Fatalf("add user: %v", err)
		}

		// Пост другого пользователя не должен влиять на ответ.
		bob := model.NewUser("bob", "hash")
		_ = userStore.AddUser(bob)
		postStore.AddPost(model.NewPost("bob's post", "news", "text", "", "text", bob))

		handler := getPostUser(userStore, postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/user/alice", nil)
		req.SetPathValue("USER_LOGIN", "alice")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rr.Code, rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("expected JSON content type, got %q", ct)
		}

		// Пустая коллекция должна кодироваться как [], а не null.
		if body := strings.TrimSpace(rr.Body.String()); body != "[]" {
			t.Errorf("expected empty array [], got %q", body)
		}

		var posts []*model.Post
		if err := json.Unmarshal(rr.Body.Bytes(), &posts); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if posts == nil {
			t.Fatal("expected non-nil slice, got null")
		}
		if len(posts) != 0 {
			t.Errorf("expected 0 posts, got %d", len(posts))
		}
	})

	t.Run("Returns only user's posts sorted by created desc", func(t *testing.T) {
		userStore := store.NewUserStore()
		postStore := store.NewPostStore()

		alice := model.NewUser("alice", "hash")
		if err := userStore.AddUser(alice); err != nil {
			t.Fatalf("add user: %v", err)
		}
		bob := model.NewUser("bob", "hash")
		if err := userStore.AddUser(bob); err != nil {
			t.Fatalf("add user: %v", err)
		}

		// Готовим посты с явными датами, чтобы проверить сортировку.
		makePost := func(title string, author *model.User, created time.Time) *model.Post {
			p := model.NewPost(title, "news", "text", "", "some text", author)
			p.Created = created.UTC().Format(time.RFC3339)
			return p
		}

		base := time.Date(2026, time.November, 2, 10, 0, 0, 0, time.UTC)

		aliceOld := makePost("alice old", alice, base)
		aliceMid := makePost("alice mid", alice, base.Add(1*time.Hour))
		aliceNew := makePost("alice new", alice, base.Add(2*time.Hour))
		bobPost := makePost("bob post", bob, base.Add(3*time.Hour)) // самый новый, но чужой

		postStore.AddPost(aliceOld)
		postStore.AddPost(aliceMid)
		postStore.AddPost(aliceNew)
		postStore.AddPost(bobPost)

		handler := getPostUser(userStore, postStore)

		req := httptest.NewRequest(http.MethodGet, "/api/user/alice", nil)
		req.SetPathValue("USER_LOGIN", "alice")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rr.Code, rr.Body.String())
		}

		var posts []*model.Post
		if err := json.Unmarshal(rr.Body.Bytes(), &posts); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if len(posts) != 3 {
			t.Fatalf("expected 3 posts, got %d", len(posts))
		}

		// Все посты должны принадлежать alice — пост bob не попал в ответ.
		for i, p := range posts {
			if p.Author == nil || p.Author.Username != "alice" {
				t.Errorf("position %d: expected author alice, got %+v", i, p.Author)
			}
		}

		// Порядок: сначала новые (created по убыванию).
		wantOrder := []string{"alice new", "alice mid", "alice old"}
		for i, want := range wantOrder {
			if posts[i].Title != want {
				t.Errorf("position %d: expected %q, got %q", i, want, posts[i].Title)
			}
		}
	})

	t.Run("Username lookup is case-insensitive", func(t *testing.T) {
		userStore := store.NewUserStore()
		postStore := store.NewPostStore()

		alice := model.NewUser("alice", "hash")
		if err := userStore.AddUser(alice); err != nil {
			t.Fatalf("add user: %v", err)
		}

		post := model.NewPost("alice post", "news", "text", "", "some text", alice)
		postStore.AddPost(post)

		handler := getPostUser(userStore, postStore)

		// Обращаемся по имени в другом регистре.
		req := httptest.NewRequest(http.MethodGet, "/api/user/ALICE", nil)
		req.SetPathValue("USER_LOGIN", "ALICE")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d, body: %s", rr.Code, rr.Body.String())
		}

		var posts []*model.Post
		if err := json.Unmarshal(rr.Body.Bytes(), &posts); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if len(posts) != 1 {
			t.Fatalf("expected 1 post, got %d", len(posts))
		}
		if posts[0].Title != "alice post" {
			t.Errorf("expected title %q, got %q", "alice post", posts[0].Title)
		}
	})
}
