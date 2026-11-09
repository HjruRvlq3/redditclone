package store

import (
	"errors"
	"sync"
	"testing"

	"redditclone/internal/model"
)

func newTestAuthor(t *testing.T) *model.User {
	t.Helper()
	return model.NewUser("alice", "hash")
}

func newTestPost(t *testing.T, author *model.User, title, category string) *model.Post {
	t.Helper()
	return model.NewPost(title, model.Category(category), "text", "", "some text", author)
}

func TestPostStore_AddPostAndGetAll(t *testing.T) {
	t.Run("Empty store returns empty non-nil slice", func(t *testing.T) {
		s := NewPostStore()
		posts := s.GetAll()
		if posts == nil {
			t.Fatal("GetAll returned nil, want empty slice")
		}
		if len(posts) != 0 {
			t.Errorf("expected 0 posts, got %d", len(posts))
		}
	})

	t.Run("Adds posts and returns all of them", func(t *testing.T) {
		s := NewPostStore()
		author := newTestAuthor(t)
		p1 := newTestPost(t, author, "one", "news")
		p2 := newTestPost(t, author, "two", "news")
		s.AddPost(p1)
		s.AddPost(p2)

		posts := s.GetAll()
		if len(posts) != 2 {
			t.Fatalf("expected 2 posts, got %d", len(posts))
		}
		got := map[string]bool{}
		for _, p := range posts {
			got[p.Id] = true
		}
		if !got[p1.Id] || !got[p2.Id] {
			t.Errorf("GetAll missing some posts: %+v", got)
		}
	})

	t.Run("Concurrent adds and reads do not race", func(t *testing.T) {
		s := NewPostStore()
		author := newTestAuthor(t)
		const n = 50
		var wg sync.WaitGroup
		wg.Add(n * 2)
		for i := 0; i < n; i++ {
			go func(i int) {
				defer wg.Done()
				s.AddPost(newTestPost(t, author, "p"+itoa(i), "news"))
			}(i)
			go func() {
				defer wg.Done()
				_ = s.GetAll()
			}()
		}
		wg.Wait()

		if got := len(s.GetAll()); got != n {
			t.Errorf("expected %d posts, got %d", n, got)
		}
	})
}

func TestPostStore_GetById(t *testing.T) {
	t.Run("Returns existing post", func(t *testing.T) {
		s := NewPostStore()
		p := newTestPost(t, newTestAuthor(t), "one", "news")
		s.AddPost(p)

		got, err := s.GetById(p.Id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != p {
			t.Errorf("returned different pointer")
		}
		// GetById не должен менять views.
		if got.Views != 0 {
			t.Errorf("views = %d, want 0", got.Views)
		}
	})

	t.Run("Returns ErrPostNotFound for unknown id", func(t *testing.T) {
		s := NewPostStore()
		_, err := s.GetById("missing")
		if !errors.Is(err, ErrPostNotFound) {
			t.Errorf("expected ErrPostNotFound, got %v", err)
		}
	})
}

func TestPostStore_GetByIdAndIncrementViews(t *testing.T) {
	t.Run("Increments views on each call", func(t *testing.T) {
		s := NewPostStore()
		p := newTestPost(t, newTestAuthor(t), "one", "news")
		s.AddPost(p)

		for want := 1; want <= 3; want++ {
			got, err := s.GetByIdAndIncrementViews(p.Id)
			if err != nil {
				t.Fatalf("iteration %d: unexpected error: %v", want, err)
			}
			if got.Views != want {
				t.Errorf("iteration %d: views = %d, want %d", want, got.Views, want)
			}
		}
	})

	t.Run("Returns ErrPostNotFound for unknown id", func(t *testing.T) {
		s := NewPostStore()
		_, err := s.GetByIdAndIncrementViews("missing")
		if !errors.Is(err, ErrPostNotFound) {
			t.Errorf("expected ErrPostNotFound, got %v", err)
		}
	})
}

func TestPostStore_GetByUser(t *testing.T) {
	t.Run("Returns only posts of the given user", func(t *testing.T) {
		s := NewPostStore()
		alice := model.NewUser("alice", "hash")
		bob := model.NewUser("bob", "hash")

		a1 := newTestPost(t, alice, "a1", "news")
		a2 := newTestPost(t, alice, "a2", "news")
		b1 := newTestPost(t, bob, "b1", "news")
		s.AddPost(a1)
		s.AddPost(a2)
		s.AddPost(b1)

		got := s.GetByUser(alice.Id)
		if len(got) != 2 {
			t.Fatalf("expected 2 alice posts, got %d", len(got))
		}
		for _, p := range got {
			if p.Author.Id != alice.Id {
				t.Errorf("wrong author: %+v", p.Author)
			}
		}
	})

	t.Run("Returns empty slice for unknown user", func(t *testing.T) {
		s := NewPostStore()
		s.AddPost(newTestPost(t, newTestAuthor(t), "one", "news"))

		got := s.GetByUser("nobody")
		if got == nil {
			t.Fatal("expected empty slice, got nil")
		}
		if len(got) != 0 {
			t.Errorf("expected 0 posts, got %d", len(got))
		}
	})
}

func TestPostStore_GetCategory(t *testing.T) {
	t.Run("Filters by category", func(t *testing.T) {
		s := NewPostStore()
		author := newTestAuthor(t)
		s.AddPost(newTestPost(t, author, "prog", "programming"))
		s.AddPost(newTestPost(t, author, "news", "news"))
		s.AddPost(newTestPost(t, author, "prog2", "programming"))

		got := s.GetCategory("programming")
		if len(got) != 2 {
			t.Fatalf("expected 2 posts, got %d", len(got))
		}
		for _, p := range got {
			if p.Category != "programming" {
				t.Errorf("wrong category: %q", p.Category)
			}
		}
	})

	t.Run("Returns empty slice for missing category", func(t *testing.T) {
		s := NewPostStore()
		s.AddPost(newTestPost(t, newTestAuthor(t), "news", "news"))

		got := s.GetCategory("unknown")
		if got == nil {
			t.Fatal("expected empty slice, got nil")
		}
		if len(got) != 0 {
			t.Errorf("expected 0 posts, got %d", len(got))
		}
	})
}

func TestPostStore_PostComment(t *testing.T) {
	t.Run("Appends comment to the post", func(t *testing.T) {
		s := NewPostStore()
		author := newTestAuthor(t)
		p := newTestPost(t, author, "one", "news")
		s.AddPost(p)

		c := model.NewComment(author, "nice!")
		got, err := s.PostComment(p.Id, c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(got.Comments))
		}
		if got.Comments[0] != c {
			t.Errorf("comment pointer differs")
		}

		// Изменение видно через GetById.
		stored, _ := s.GetById(p.Id)
		if len(stored.Comments) != 1 {
			t.Errorf("stored comments = %d, want 1", len(stored.Comments))
		}
	})

	t.Run("Returns ErrPostNotFound for unknown post", func(t *testing.T) {
		s := NewPostStore()
		_, err := s.PostComment("missing", model.NewComment(newTestAuthor(t), "hi"))
		if !errors.Is(err, ErrPostNotFound) {
			t.Errorf("expected ErrPostNotFound, got %v", err)
		}
	})
}

func TestPostStore_DeleteComment(t *testing.T) {
	t.Run("Removes existing comment", func(t *testing.T) {
		s := NewPostStore()
		author := newTestAuthor(t)
		p := newTestPost(t, author, "one", "news")
		s.AddPost(p)
		c := model.NewComment(author, "hello")
		_, _ = s.PostComment(p.Id, c)

		got, err := s.DeleteComment(p.Id, c.Id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Comments) != 0 {
			t.Errorf("expected 0 comments, got %d", len(got.Comments))
		}
	})

	t.Run("Returns ErrPostNotFound for unknown post", func(t *testing.T) {
		s := NewPostStore()
		_, err := s.DeleteComment("missing", "some-comment")
		if !errors.Is(err, ErrPostNotFound) {
			t.Errorf("expected ErrPostNotFound, got %v", err)
		}
	})

	t.Run("Returns ErrCommentNotFound for unknown comment", func(t *testing.T) {
		s := NewPostStore()
		p := newTestPost(t, newTestAuthor(t), "one", "news")
		s.AddPost(p)
		_, err := s.DeleteComment(p.Id, "missing")
		if !errors.Is(err, ErrCommentNotFound) {
			t.Errorf("expected ErrCommentNotFound, got %v", err)
		}
	})

	t.Run("Cannot delete same comment twice", func(t *testing.T) {
		s := NewPostStore()
		author := newTestAuthor(t)
		p := newTestPost(t, author, "one", "news")
		s.AddPost(p)
		c := model.NewComment(author, "hello")
		_, _ = s.PostComment(p.Id, c)

		if _, err := s.DeleteComment(p.Id, c.Id); err != nil {
			t.Fatalf("first delete: %v", err)
		}
		if _, err := s.DeleteComment(p.Id, c.Id); !errors.Is(err, ErrCommentNotFound) {
			t.Errorf("second delete: expected ErrCommentNotFound, got %v", err)
		}
	})
}

func TestPostStore_Vote(t *testing.T) {
	t.Run("New vote from another user increases score", func(t *testing.T) {
		s := NewPostStore()
		author := newTestAuthor(t)
		p := newTestPost(t, author, "one", "news")
		s.AddPost(p)
		// NewPost ставит голос автора: score=1, upvotePct=100.

		bob := model.NewUser("bob", "hash")
		got, err := s.Vote(p.Id, bob.Id, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Score != 2 {
			t.Errorf("score = %d, want 2", got.Score)
		}
		if got.UpvotePercentage != 100 {
			t.Errorf("upvotePercentage = %d, want 100", got.UpvotePercentage)
		}
		if len(got.Votes) != 2 {
			t.Errorf("expected 2 votes, got %d", len(got.Votes))
		}
	})

	t.Run("Repeated upvote by same user does not add a vote", func(t *testing.T) {
		s := NewPostStore()
		p := newTestPost(t, newTestAuthor(t), "one", "news")
		s.AddPost(p)
		bob := model.NewUser("bob", "hash")

		for i := 0; i < 3; i++ {
			if _, err := s.Vote(p.Id, bob.Id, 1); err != nil {
				t.Fatalf("iter %d: %v", i, err)
			}
		}
		stored, _ := s.GetById(p.Id)
		if stored.Score != 2 {
			t.Errorf("score = %d, want 2", stored.Score)
		}
		if len(stored.Votes) != 2 {
			t.Errorf("expected 2 votes, got %d", len(stored.Votes))
		}
	})

	t.Run("Changing vote flips score without adding entries", func(t *testing.T) {
		s := NewPostStore()
		p := newTestPost(t, newTestAuthor(t), "one", "news")
		s.AddPost(p)
		bob := model.NewUser("bob", "hash")

		// upvote, затем downvote
		if _, err := s.Vote(p.Id, bob.Id, 1); err != nil {
			t.Fatalf("upvote: %v", err)
		}
		got, err := s.Vote(p.Id, bob.Id, -1)
		if err != nil {
			t.Fatalf("downvote: %v", err)
		}
		if got.Score != 0 {
			t.Errorf("score = %d, want 0", got.Score)
		}
		if got.UpvotePercentage != 50 {
			t.Errorf("upvotePercentage = %d, want 50", got.UpvotePercentage)
		}
		if len(got.Votes) != 2 {
			t.Errorf("expected 2 votes, got %d", len(got.Votes))
		}
	})

	t.Run("UpvotePercentage is floored", func(t *testing.T) {
		s := NewPostStore()
		author := newTestAuthor(t)
		p := newTestPost(t, author, "one", "news")
		s.AddPost(p) // author: +1

		bob := model.NewUser("bob", "hash")
		charlie := model.NewUser("charlie", "hash")
		if _, err := s.Vote(p.Id, bob.Id, 1); err != nil {
			t.Fatalf("bob vote: %v", err)
		}
		got, err := s.Vote(p.Id, charlie.Id, -1)
		if err != nil {
			t.Fatalf("charlie vote: %v", err)
		}
		// 2 up, 1 down: score = 1, pct = floor(100*2/3) = 66.
		if got.Score != 1 {
			t.Errorf("score = %d, want 1", got.Score)
		}
		if got.UpvotePercentage != 66 {
			t.Errorf("upvotePercentage = %d, want 66", got.UpvotePercentage)
		}
	})

	t.Run("Returns ErrPostNotFound for unknown post", func(t *testing.T) {
		s := NewPostStore()
		_, err := s.Vote("missing", "user-id", 1)
		if !errors.Is(err, ErrPostNotFound) {
			t.Errorf("expected ErrPostNotFound, got %v", err)
		}
	})
}

func TestPostStore_Unvote(t *testing.T) {
	t.Run("Removes existing vote", func(t *testing.T) {
		s := NewPostStore()
		author := newTestAuthor(t)
		p := newTestPost(t, author, "one", "news")
		s.AddPost(p) // votes: [alice:+1]

		got, err := s.Unvote(p.Id, author.Id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Score != 0 {
			t.Errorf("score = %d, want 0", got.Score)
		}
		if got.UpvotePercentage != 0 {
			t.Errorf("upvotePercentage = %d, want 0", got.UpvotePercentage)
		}
		if got.Votes == nil {
			t.Fatal("votes = nil, want empty slice")
		}
		if len(got.Votes) != 0 {
			t.Errorf("expected 0 votes, got %d", len(got.Votes))
		}
	})

	t.Run("Unvote by user who did not vote is a no-op", func(t *testing.T) {
		s := NewPostStore()
		author := newTestAuthor(t)
		p := newTestPost(t, author, "one", "news")
		s.AddPost(p) // votes: [alice:+1]

		bob := model.NewUser("bob", "hash")
		got, err := s.Unvote(p.Id, bob.Id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Score != 1 {
			t.Errorf("score = %d, want 1", got.Score)
		}
		if len(got.Votes) != 1 || got.Votes[0].User != author.Id {
			t.Errorf("alice's vote changed: %+v", got.Votes)
		}
	})

	t.Run("Removing one of two votes keeps score consistent", func(t *testing.T) {
		s := NewPostStore()
		author := newTestAuthor(t)
		p := newTestPost(t, author, "one", "news")
		s.AddPost(p)
		bob := model.NewUser("bob", "hash")
		if _, err := s.Vote(p.Id, bob.Id, 1); err != nil {
			t.Fatalf("bob vote: %v", err)
		}

		got, err := s.Unvote(p.Id, bob.Id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Score != 1 {
			t.Errorf("score = %d, want 1", got.Score)
		}
		if got.UpvotePercentage != 100 {
			t.Errorf("upvotePercentage = %d, want 100", got.UpvotePercentage)
		}
		if len(got.Votes) != 1 || got.Votes[0].User != author.Id {
			t.Errorf("unexpected votes: %+v", got.Votes)
		}
	})

	t.Run("Returns ErrPostNotFound for unknown post", func(t *testing.T) {
		s := NewPostStore()
		_, err := s.Unvote("missing", "user-id")
		if !errors.Is(err, ErrPostNotFound) {
			t.Errorf("expected ErrPostNotFound, got %v", err)
		}
	})
}

func TestPostStore_DeletePost(t *testing.T) {
	t.Run("Removes existing post", func(t *testing.T) {
		s := NewPostStore()
		author := newTestAuthor(t)
		p1 := newTestPost(t, author, "one", "news")
		p2 := newTestPost(t, author, "two", "news")
		s.AddPost(p1)
		s.AddPost(p2)

		if err := s.DeletePost(p1.Id); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := s.GetById(p1.Id); !errors.Is(err, ErrPostNotFound) {
			t.Errorf("deleted post still retrievable: %v", err)
		}
		// Другой пост не задет.
		if _, err := s.GetById(p2.Id); err != nil {
			t.Errorf("unrelated post affected: %v", err)
		}
	})

	t.Run("Returns ErrPostNotFound for unknown post", func(t *testing.T) {
		s := NewPostStore()
		if err := s.DeletePost("missing"); !errors.Is(err, ErrPostNotFound) {
			t.Errorf("expected ErrPostNotFound, got %v", err)
		}
	})

	t.Run("Cannot delete post twice", func(t *testing.T) {
		s := NewPostStore()
		p := newTestPost(t, newTestAuthor(t), "one", "news")
		s.AddPost(p)

		if err := s.DeletePost(p.Id); err != nil {
			t.Fatalf("first delete: %v", err)
		}
		if err := s.DeletePost(p.Id); !errors.Is(err, ErrPostNotFound) {
			t.Errorf("second delete: expected ErrPostNotFound, got %v", err)
		}
	})
}
