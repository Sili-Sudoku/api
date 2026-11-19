package list

import (
	"context"

	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Endpoint struct {
	db *DB
}

func New(db *DB) *Endpoint { return &Endpoint{db: db} }

type DB struct {
	Puzzle PuzzleDB
	User   UserDB
}

type PuzzleDB interface {
	List(ctx context.Context, f model.Filter) ([]*model.Puzzle, error)
}

type UserDB interface {
	FindByName(ctx context.Context, name string) (*accounts.User, error)
	Usernames(ctx context.Context, ids []int64) (map[int64]string, error)
}
