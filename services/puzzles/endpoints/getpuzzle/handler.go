package getpuzzle

import (
	"context"

	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Endpoint struct {
	auth Auth
	db   *DB
}

func New(auth Auth, db *DB) *Endpoint { return &Endpoint{auth: auth, db: db} }

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
