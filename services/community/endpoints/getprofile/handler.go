package getprofile

import (
	"context"

	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
)

type Endpoint struct {
	auth Auth
	db   *DB
}

func New(auth Auth, db *DB) *Endpoint { return &Endpoint{auth: auth, db: db} }

// Auth answers for signed-out callers too: FindUser returns nil then.
type Auth interface {
	FindUser(ctx context.Context) *accounts.User
}

type DB struct {
	Follow FollowDB
	User   UserDB
}

type FollowDB interface {
	Counts(ctx context.Context, userID int64) (followers, following int, err error)
	Exists(ctx context.Context, followerID, creatorID int64) (bool, error)
}

type UserDB interface {
	FindByName(ctx context.Context, name string) (*accounts.User, error)
}
