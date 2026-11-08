package model

import (
	"time"

	"github.com/google/uuid"
)

type Category string

type Post struct {
	Id               string     `json:"id"`
	Title            string     `json:"title"`
	Category         Category   `json:"category"`
	Type             string     `json:"type"`
	URL              string     `json:"url,omitempty"`
	Text             string     `json:"text,omitempty"`
	Author           *User      `json:"author"`
	Score            int        `json:"score"`
	Votes            []*Vote    `json:"votes"`
	Comments         []*Comment `json:"comments"`
	Created          string     `json:"created"`
	Views            int        `json:"views"`
	UpvotePercentage int        `json:"upvotePercentage"`
}

func NewPost(title string, category Category, postType, url, text string, author *User) *Post {
	return &Post{
		Id:               uuid.New().String(),
		Title:            title,
		Category:         category,
		Type:             postType,
		URL:              url,
		Text:             text,
		Author:           author,
		Score:            1,
		Votes:            []*Vote{{User: author.Id, Vote: 1}},
		Comments:         []*Comment{},
		Created:          time.Now().UTC().Format(time.RFC3339),
		Views:            0,
		UpvotePercentage: 100,
	}
}
