package postcomment

import (
	"context"

	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/community/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Endpoint struct {
	auth  Auth
	db    *DB
	limit Limiter
}

func New(auth Auth, db *DB, limit Limiter) *Endpoint {
	return &Endpoint{auth: auth, db: db, limit: limit}
}

type Auth interface {
	GetUser(ctx context.Context) (*accounts.User, error)
}

type DB struct {
	Comment CommentDB
	Puzzle  PuzzleDB
}

type CommentDB interface {
	Create(ctx context.Context, c *model.Comment) error
}

type Limiter interface {
	Allow(key string) bool
}

type PuzzleDB interface {
	Find(ctx context.Context, id string) (*puzzles.Puzzle, error)
}
