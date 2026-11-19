package listfollowing

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
	List(ctx context.Context, followerID int64, limit int) ([]int64, error)
}

type UserDB interface {
	Usernames(ctx context.Context, ids []int64) (map[int64]string, error)
}
