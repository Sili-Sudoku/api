package record

import (
	"context"

	"github.com/Sili-Sudoku/api/services/history/model"
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
	SolveHistory SolveHistoryDB
}

type SolveHistoryDB interface {
	Upsert(ctx context.Context, userID int64, e model.Entry) (model.Entry, error)
}
