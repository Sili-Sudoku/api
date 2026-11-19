package deletecomment

import (
	"context"

	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/community/model"
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
	Comment CommentDB
}

type CommentDB interface {
	Delete(ctx context.Context, id int64) error
	Find(ctx context.Context, id int64) (*model.Comment, error)
}
