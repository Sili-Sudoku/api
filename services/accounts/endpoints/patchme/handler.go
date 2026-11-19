package patchme

import (
	"context"

	"github.com/Sili-Sudoku/api/services/accounts/model"
)

type Endpoint struct {
	auth Auth
	db   *DB
}

func New(auth Auth, db *DB) *Endpoint { return &Endpoint{auth: auth, db: db} }

type Auth interface {
	GetUser(ctx context.Context) (*model.User, error)
}

type DB struct {
	User UserDB
}

type UserDB interface {
	SetBio(ctx context.Context, id int64, bio string) error
}
