// Package model holds what the community endpoints and the Postgres clients share.
package model

import (
	"errors"
	"time"

	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

const (
	MaxComment = 2000
	// ReportLimit is how many reports hide a comment until an admin reviews it.
	ReportLimit = 3
	// MaxFollowing bounds the feed query and the table.
	MaxFollowing = 500
)

var ErrNotFound = errors.New("not found")

// Rating is one user's vote. 0 means "not given"; the other values are 1 to 5.
type Rating struct {
	Stars, Difficulty int
}

type Comment struct {
	ID         int64
	PuzzleID   string
	UserID     int64
	Body       string
	Reports    int
	Moderation puzzles.Moderation
	CreatedAt  time.Time
}

// Visible reports whether lists may show the comment. The rule is the same as for puzzles.
func (c *Comment) Visible() bool {
	if c.Moderation == puzzles.ModHidden {
		return false
	}

	return c.Moderation == puzzles.ModCleared || c.Reports < ReportLimit
}

// CommentCursor is a position in the newest-first order of comments.
type CommentCursor struct {
	CreatedAt time.Time
	ID        int64
}
