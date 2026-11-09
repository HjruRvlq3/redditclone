package jwtconfig

import (
	"time"

	"redditclone/internal/model"

	"github.com/golang-jwt/jwt/v5"
)

// Claims ...
type Claims struct {
	User *model.User `json:"user"`
	jwt.RegisteredClaims
}

// NewClaims ...
func NewClaims(u *model.User) *Claims {
	now := time.Now()
	return &Claims{
		User: u,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
		},
	}
}
