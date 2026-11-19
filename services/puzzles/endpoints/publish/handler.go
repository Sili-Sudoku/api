package publish

import (
	"context"

	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Endpoint struct {
	auth       Auth
	db         *DB
	limit      Limiter
	trustProxy bool
	validator  Validator
}

func New(auth Auth, db *DB, limit Limiter, validator Validator, trustProxy bool) *Endpoint {
	return &Endpoint{auth: auth, db: db, limit: limit, trustProxy: trustProxy, validator: validator}
}

type Auth interface {
	FindUser(ctx context.Context) *accounts.User
}

type DB struct {
	Puzzle PuzzleDB
}

type Limiter interface {
	Allow(key string) bool
}

type PuzzleDB interface {
	Create(ctx context.Context, p *model.Puzzle) (created bool, err error)
}

// Validator is run in the background for every published puzzle: validation never runs inside a request.
type Validator interface {
	Validate(ctx context.Context, id string) error
}
