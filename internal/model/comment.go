package model

import (
	"time"

	"github.com/google/uuid"
)

type Comment struct {
	Id      string    `json:"id"`
	Author  *User     `json:"author"`
	Body    string    `json:"body"`
	Created time.Time `json:"created"`
}

func NewComment(author *User, body string) *Comment {
	return &Comment{
		Id:      uuid.New().String(),
		Author:  author,
		Body:    body,
		Created: time.Now().UTC(),
	}
}
