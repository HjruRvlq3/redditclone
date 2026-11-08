package store

import (
	"errors"
	"strings"
	"sync"

	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/model"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
)

// UserStore список пользователей
type UserStore struct {
	mu    sync.RWMutex
	items map[string]*model.User
}

// NewUserStore создает новый список
func NewUserStore() *UserStore {
	return &UserStore{
		items: make(map[string]*model.User),
	}
}

// AddUser добавить пользователя в список
func (s *UserStore) AddUser(u *model.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, user := range s.items {
		if strings.EqualFold(user.Username, u.Username) {
			return ErrUserAlreadyExists
		}
	}

	s.items[u.Id] = u
	return nil
}

// GetById отдает юзера по id
func (s *UserStore) GetById(id string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.items[id]
	if !ok {
		return nil, ErrUserNotFound
	}

	return u, nil
}

// GetByUsername отдает юзера по логину
func (s *UserStore) GetByUsername(username string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.items {
		if strings.EqualFold(user.Username, username) {
			return user, nil
		}
	}

	return nil, ErrUserNotFound
}
