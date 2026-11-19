package login

import (
	"context"
	"time"

	"github.com/Sili-Sudoku/api/libs/password"
	"github.com/Sili-Sudoku/api/services/accounts/model"
)

type Endpoint struct {
	cfg Config
	db  *DB
	// dummy is checked when the username is unknown, so a miss takes as long as a hit.
	dummy     string
	limitIP   Limiter
	limitUser Limiter
}

func New(db *DB, limitIP, limitUser Limiter, cfg Config) *Endpoint {
	dummy, _ := password.Hash("dummy-password-for-timing")
	return &Endpoint{cfg: cfg, db: db, dummy: dummy, limitIP: limitIP, limitUser: limitUser}
}

type Config struct {
	InsecureCookie bool
	TrustProxy     bool
}

type DB struct {
	Session SessionDB
	User    UserDB
}

type Limiter interface {
	Allow(key string) bool
}

type SessionDB interface {
	Create(ctx context.Context, hash [32]byte, userID int64, expires time.Time) error
}

type UserDB interface {
	FindByName(ctx context.Context, name string) (*model.User, error)
}
