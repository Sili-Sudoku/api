// Package model holds what the account endpoints and the Postgres clients share.
package model

import (
	"errors"
	"regexp"
	"time"
)

var (
	ErrUsernameTaken = errors.New("username taken")
	ErrNotFound      = errors.New("not found")
)

const (
	MinPassword = 10
	MaxPassword = 200 // argon2 costs memory, so an unbounded password is a cheap attack
	MaxBio      = 500 // in characters
)

var UsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,24}$`)

type User struct {
	ID           int64
	Username     string
	Email        string
	Bio          string
	IsAdmin      bool
	CreatedAt    time.Time
	PasswordHash string
}
