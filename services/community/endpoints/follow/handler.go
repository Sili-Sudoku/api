package follow

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
	Follow FollowDB
	User   UserDB
}

type FollowDB interface {
	Counts(ctx context.Context, userID int64) (followers, following int, err error)
	Create(ctx context.Context, followerID, creatorID int64) error
	Exists(ctx context.Context, followerID, creatorID int64) (bool, error)
}

type UserDB interface {
	FindByName(ctx context.Context, name string) (*accounts.User, error)
}
