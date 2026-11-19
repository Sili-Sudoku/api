// Package model holds what the puzzle endpoints, the Postgres client and the community package share.
package model

import (
	"errors"
	"time"

	"github.com/Sili-Sudoku/api/libs/validate"
)

const MaxPuzzleBytes = 256 << 10

// ReportLimit is how many reports hide a puzzle from lists until an admin reviews it.
const ReportLimit = 3

type Status string

const (
	Pending   Status = "pending"
	Published Status = "published"
	Rejected  Status = "rejected"
)

// Moderation is the admin's decision. See migrations/0003.
type Moderation string

const (
	ModNone    Moderation = "none"
	ModHidden  Moderation = "hidden"
	ModCleared Moderation = "cleared"
)

var ErrNotFound = errors.New("puzzle not found")

type Puzzle struct {
	ID         string
	Body       []byte // the PuzzleDefinition JSON, stored as received
	Status     Status
	Verdict    *validate.Verdict
	Reports    int
	Moderation Moderation
	OwnerID    int64 // 0 = no owner (anonymous, or the owner's account is gone)
	CreatedAt  time.Time
	TokenHash  [32]byte

	// Copied from the body at publish time, for lists. All plain text.
	Title      string
	Author     string
	Source     string
	Difficulty string
	Tags       []string
	Variants   []string

	Ratings RatingStats
}

// RatingStats are the sums and counts of the ratings of a puzzle.
type RatingStats struct {
	StarsCount, StarsSum           int
	DifficultyCount, DifficultySum int
}

// TopScore orders puzzles by rating. It is the mean of the stars with a prior of two votes at 3 stars, so one
// 5-star vote does not beat fifty 4.8-star votes. It is an integer so that Go and SQL sort the same way.
func (r RatingStats) TopScore() int { return (r.StarsSum + 6) * 1000 / (r.StarsCount + 2) }

// Visible reports whether lists may show the puzzle.
func (p *Puzzle) Visible() bool {
	if p.Status != Published || p.Moderation == ModHidden {
		return false
	}

	return p.Moderation == ModCleared || p.Reports < ReportLimit
}

// Filter selects puzzles for a list. Only published, visible puzzles are ever listed.
type Filter struct {
	OwnerIDs   []int64 // nil = any owner
	Tag        string
	Variant    string
	Difficulty string
	Limit      int
	Top        bool    // order by rating instead of by date
	Offset     int     // for Top only: how many puzzles of the order to skip
	After      *Cursor // for the newest-first order: nil = from the newest
}

// Cursor is a position in the newest-first order, or an offset in the Top order.
type Cursor struct {
	CreatedAt time.Time `json:"t"`
	ID        string    `json:"id"`
	Offset    int       `json:"o,omitempty"` // Top order only
}

// Valid is what a cursor from a client must satisfy.
func (c *Cursor) Valid() bool { return (c.ID != "" || c.Offset > 0) && c.Offset >= 0 }
