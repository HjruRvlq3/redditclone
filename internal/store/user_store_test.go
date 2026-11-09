package store

import (
	"errors"
	"sync"
	"testing"

	"redditclone/internal/model"
)

func TestUserStore_AddUser(t *testing.T) {
	t.Run("Adds a new user", func(t *testing.T) {
		s := NewUserStore()
		u := model.NewUser("alice", "hash")

		if err := s.AddUser(u); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := s.GetById(u.Id)
		if err != nil {
			t.Fatalf("expected to find user, got error: %v", err)
		}
		if got != u {
			t.Errorf("stored pointer differs from added one")
		}
	})

	t.Run("Duplicate username is rejected case-insensitively", func(t *testing.T) {
		s := NewUserStore()
		if err := s.AddUser(model.NewUser("alice", "hash")); err != nil {
			t.Fatalf("setup: %v", err)
		}

		cases := []string{"alice", "ALICE", "Alice", "aLiCe"}
		for _, username := range cases {
			err := s.AddUser(model.NewUser(username, "hash2"))
			if !errors.Is(err, ErrUserAlreadyExists) {
				t.Errorf("username %q: expected ErrUserAlreadyExists, got %v", username, err)
			}
		}
	})

	t.Run("Different usernames are accepted", func(t *testing.T) {
		s := NewUserStore()
		for _, username := range []string{"alice", "bob", "charlie"} {
			if err := s.AddUser(model.NewUser(username, "hash")); err != nil {
				t.Fatalf("add %q: %v", username, err)
			}
		}
	})

	t.Run("Concurrent adds of distinct users do not race", func(t *testing.T) {
		s := NewUserStore()
		const n = 50
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func(i int) {
				defer wg.Done()
				_ = s.AddUser(model.NewUser("user"+itoa(i), "hash"))
			}(i)
		}
		wg.Wait()

		// Все пользователи с уникальными именами должны быть добавлены.
		for i := 0; i < n; i++ {
			if _, err := s.GetByUsername("user" + itoa(i)); err != nil {
				t.Errorf("user%d not found: %v", i, err)
			}
		}
	})
}

func TestUserStore_GetById(t *testing.T) {
	t.Run("Returns existing user", func(t *testing.T) {
		s := NewUserStore()
		u := model.NewUser("alice", "hash")
		_ = s.AddUser(u)

		got, err := s.GetById(u.Id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Id != u.Id || got.Username != "alice" {
			t.Errorf("got %+v, want %+v", got, u)
		}
	})

	t.Run("Returns ErrUserNotFound for unknown id", func(t *testing.T) {
		s := NewUserStore()
		_, err := s.GetById("missing")
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestUserStore_GetByUsername(t *testing.T) {
	t.Run("Lookup is case-insensitive", func(t *testing.T) {
		s := NewUserStore()
		u := model.NewUser("Alice", "hash")
		_ = s.AddUser(u)

		for _, query := range []string{"Alice", "alice", "ALICE", "aLiCe"} {
			got, err := s.GetByUsername(query)
			if err != nil {
				t.Errorf("query %q: unexpected error: %v", query, err)
				continue
			}
			if got.Id != u.Id {
				t.Errorf("query %q: got id %q, want %q", query, got.Id, u.Id)
			}
		}
	})

	t.Run("Returns ErrUserNotFound for unknown username", func(t *testing.T) {
		s := NewUserStore()
		_, err := s.GetByUsername("nobody")
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})
}

// itoa — маленький помощник, чтобы не тянуть strconv в тест.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [10]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
