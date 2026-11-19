package report

import (
	"context"

	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
)

type Endpoint struct {
	auth Auth
	db   *DB
}

func New(auth Auth, db *DB) *Endpoint { return &Endpoint{auth: auth, db: db} }

type Auth interface {
	GetUser(ctx context.Context) (*accounts.User, error)
}

type DB struct {
	Puzzle PuzzleDB
}

type PuzzleDB interface {
	AddReport(ctx context.Context, id string, userID int64) (int, error)
}
