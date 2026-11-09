package jwtconfig

import (
	"errors"
	"fmt"

	"redditclone/internal/model"

	"github.com/golang-jwt/jwt/v5"
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

// ParseToken проверка токена
func (m *Manager) ParseToken(tokenString string) (*model.User, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (any, error) {
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.User.Id == "" || claims.User == nil || claims.User.Username == "" {
		return nil, ErrInvalidToken
	}

	return claims.User, nil
}
