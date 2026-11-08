package store

import (
	"errors"
	"slices"
	"sync"

	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/model"
)

var (
	ErrPostNotFound    = errors.New("post not found")
	ErrCommentNotFound = errors.New("comment not found")
)

// PostStore список постов
type PostStore struct {
	mu    sync.RWMutex
	items map[string]*model.Post
}

// NewPostStore новый список постов
func NewPostStore() *PostStore {
	return &PostStore{
		items: make(map[string]*model.Post),
	}
}

// AddPost добавление нового поста
func (s *PostStore) AddPost(p *model.Post) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[p.Id] = p
}

// GetAll возвращает все посты
func (s *PostStore) GetAll() []*model.Post {
	s.mu.RLock()
	defer s.mu.RUnlock()

	postList := make([]*model.Post, 0, len(s.items))

	for _, p := range s.items {
		postList = append(postList, p)
	}

	return postList
}

// GetById возвращает пост по id
func (s *PostStore) GetById(id string) (*model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.items[id]
	if !ok {
		return nil, ErrPostNotFound
	}

	return p, nil
}

// GetByIdAndIncrementViews возвращает пост по id и увеличивает счетчик просмотров
func (s *PostStore) GetByIdAndIncrementViews(id string) (*model.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.items[id]
	if !ok {
		return nil, ErrPostNotFound
	}

	p.Views++

	return p, nil
}

// GetByUser возвращает посты юзера
func (s *PostStore) GetByUser(userId string) []*model.Post {
	s.mu.RLock()
	defer s.mu.RUnlock()

	postList := make([]*model.Post, 0)

	for _, post := range s.items {
		if post.Author.Id == userId {
			postList = append(postList, post)
		}
	}

	return postList
}

// GetCategory возвращает посты указанной категории
func (s *PostStore) GetCategory(c model.Category) []*model.Post {
	s.mu.RLock()
	defer s.mu.RUnlock()

	postList := make([]*model.Post, 0, len(s.items))

	for _, p := range s.items {
		if p.Category == c {
			postList = append(postList, p)
		}
	}

	return postList
}

// PostComment добавление коммента
func (s *PostStore) PostComment(postId string, c *model.Comment) (*model.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.items[postId]
	if !ok {
		return nil, ErrPostNotFound
	}

	p.Comments = append(p.Comments, c)

	return p, nil
}

// DeleteComment удаление коммента
func (s *PostStore) DeleteComment(postId string, commentId string) (*model.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.items[postId]
	if !ok {
		return nil, ErrPostNotFound
	}

	for i, comment := range p.Comments {
		if comment.Id == commentId {
			p.Comments = slices.Delete(p.Comments, i, i+1)
			return p, nil
		}
	}

	return nil, ErrCommentNotFound
}

// Vote устанавливает или заменяет голос пользователя за пост
func (s *PostStore) Vote(postID, userID string, vote int) (*model.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.items[postID]
	if !ok {
		return nil, ErrPostNotFound
	}

	found := false
	for _, v := range p.Votes {
		if v.User == userID {
			v.Vote = vote
			found = true
			break
		}
	}
	if !found {
		p.Votes = append(p.Votes, &model.Vote{User: userID, Vote: vote})
	}

	recalcVotes(p)
	return p, nil
}

// Unvote удаляет голос пользователя
func (s *PostStore) Unvote(postID, userID string) (*model.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.items[postID]
	if !ok {
		return nil, ErrPostNotFound
	}

	for i, v := range p.Votes {
		if v.User == userID {
			p.Votes = slices.Delete(p.Votes, i, i+1)
			break
		}
	}

	if len(p.Votes) == 0 {
		p.Votes = []*model.Vote{}
	}

	recalcVotes(p)
	return p, nil
}

// DeletePost удаляет пост
func (s *PostStore) DeletePost(postID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[postID]; !ok {
		return ErrPostNotFound
	}

	delete(s.items, postID)
	return nil
}

// recalcVotes пересчитывает score и upvotePercentage
func recalcVotes(p *model.Post) {
	if len(p.Votes) == 0 {
		p.Score = 0
		p.UpvotePercentage = 0
		return
	}

	score := 0
	upvotes := 0
	for _, v := range p.Votes {
		score += v.Vote
		if v.Vote == 1 {
			upvotes++
		}
	}

	p.Score = score
	p.UpvotePercentage = 100 * upvotes / len(p.Votes)
}
