package model

import (
	"crypto/sha256"
	"crypto/subtle"
)

// HasToken reports whether token is the delete token that the publisher of the puzzle was given.
func (p *Puzzle) HasToken(token string) bool {
	if token == "" {
		return false
	}
	got := sha256.Sum256([]byte(token))

	return subtle.ConstantTimeCompare(got[:], p.TokenHash[:]) == 1
}
