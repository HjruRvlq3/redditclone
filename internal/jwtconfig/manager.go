package jwtconfig

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"gitlab.com/vk-golang/lectures/seminars/01_reddit_clone/redditclone/internal/model"
)

var ErrInvalidToken = errors.New("invalid token")

// Manager ...
type Manager struct {
	secret []byte
}

// NewManager ...
func NewManager(secret []byte) *Manager {
	return &Manager{secret: secret}

}

// GenerateToken ...
func (m *Manager) GenerateToken(u *model.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, NewClaims(u))

	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// ParseToken ...
func (m *Manager) ParseToken(tokenString string) (*model.User, error) {

}
