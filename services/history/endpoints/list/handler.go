package list

import (
	"context"

	"github.com/Sili-Sudoku/api/services/history/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Endpoint struct {
	auth Auth
	db   *DB
}

func New(auth Auth, db *DB) *Endpoint { return &Endpoint{auth: auth, db: db} }

type Auth interface {
	GetUserID(ctx context.Context) (int64, error)
}

type DB struct {
	Puzzle       PuzzleDB
	SolveHistory SolveHistoryDB
}

type PuzzleDB interface {
	Find(ctx context.Context, id string) (*puzzles.Puzzle, error)
}

type SolveHistoryDB interface {
	List(ctx context.Context, userID int64, after *model.Cursor, limit int) ([]model.Entry, error)
}
