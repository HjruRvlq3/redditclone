package model

import "github.com/google/uuid"

type User struct {
	Id           string `json:"id"`
	Username     string `json:"username"`
	Admin        bool   `json:"admin,omitempty"`
	PasswordHash string `json:"-"`
}

func NewUser(username string, passwordHash string) *User {
	id := uuid.New().String()
	return &User{
		Id:           id,
		Username:     username,
		Admin:        false,
		PasswordHash: passwordHash,
	}
}
