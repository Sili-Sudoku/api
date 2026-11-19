package solvestats

import (
	"context"

	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Endpoint struct {
	db *DB
}

func New(db *DB) *Endpoint { return &Endpoint{db: db} }

type DB struct {
	Puzzle       PuzzleDB
	SolveHistory SolveHistoryDB
}

type PuzzleDB interface {
	Find(ctx context.Context, id string) (*puzzles.Puzzle, error)
}

// SolveHistoryDB gives the best times of the users who sync their history. history/postgres implements it.
type SolveHistoryDB interface {
	SolveSeconds(ctx context.Context, puzzleID string) ([]int, error)
}
