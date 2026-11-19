package model

import (
	"time"

	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

// RatingView is the rating of a puzzle as the client sees it. Mine is nil when the caller has not rated it.
type RatingView struct {
	puzzles.RatingSummary
	Mine *MyRating `json:"mine"`
}

type MyRating struct {
	Stars      int `json:"stars"`
	Difficulty int `json:"difficulty"`
}

func NewRatingView(stats puzzles.RatingStats, mine *Rating) RatingView {
	v := RatingView{RatingSummary: puzzles.NewRatingSummary(stats)}
	if mine != nil {
		v.Mine = &MyRating{Stars: mine.Stars, Difficulty: mine.Difficulty}
	}

	return v
}

// CommentView is a comment as the client sees it. Author is nil when the account is gone.
type CommentView struct {
	ID        int64     `json:"id"`
	Author    *string   `json:"author"`
	Text      string    `json:"text"` // plain text; the client must not render it as HTML
	CreatedAt time.Time `json:"createdAt"`
	Mine      bool      `json:"mine"`
}

// NewCommentView shows c to the signed-in user userID (0 when nobody is signed in). names maps user ids to usernames.
func NewCommentView(c *Comment, names map[int64]string, userID int64) CommentView {
	v := CommentView{ID: c.ID, Text: c.Body, CreatedAt: c.CreatedAt, Mine: userID != 0 && userID == c.UserID}
	if name, ok := names[c.UserID]; ok {
		v.Author = &name
	}

	return v
}
