// Package model holds what the history endpoints and the Postgres client share.
package model

import (
	"errors"
	"regexp"
	"time"
)

const (
	// MaxEntries is the most puzzles one user can store. It bounds the table against abuse.
	MaxEntries = 5000
	// MaxSeconds is 100 days: a bigger number is a mistake.
	MaxSeconds = 8640000
)

var (
	ErrFull = errors.New("history is full")

	IDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

type Entry struct {
	PuzzleID  string
	SolvedAt  *time.Time
	Seconds   *int // the best time
	UpdatedAt time.Time
}

// Cursor is a position in the most-recently-updated-first order.
type Cursor struct {
	UpdatedAt time.Time `json:"t"`
	PuzzleID  string    `json:"id"`
}
