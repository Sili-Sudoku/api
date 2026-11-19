package deleteall

import "context"

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
	DeleteAll(ctx context.Context, userID int64) error
}
