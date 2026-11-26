package getpuzzle

import (
	"context"
	"time"

	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

// ValidateTimeout bounds the retry of a pending puzzle inside a read.
const ValidateTimeout = 30 * time.Second

type Endpoint struct {
	auth      Auth
	db        *DB
	validator Validator
}

func New(auth Auth, db *DB, validator Validator) *Endpoint {
	return &Endpoint{auth: auth, db: db, validator: validator}
}

// Validator is the engine run. A puzzle is pending when its publish request could not finish the check (the engine
// failed or timed out), so reading it tries again. The validator ignores a puzzle that is already being checked.
type Validator interface {
	Validate(ctx context.Context, id string) error
}

type Auth interface {
	FindUser(ctx context.Context) *accounts.User
}

type DB struct {
	Puzzle PuzzleDB
	User   UserDB
}

type PuzzleDB interface {
	Find(ctx context.Context, id string) (*model.Puzzle, error)
}

type UserDB interface {
	Usernames(ctx context.Context, ids []int64) (map[int64]string, error)
}
