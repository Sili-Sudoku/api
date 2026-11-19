package signup

import (
	"context"
	"time"

	"github.com/Sili-Sudoku/api/services/accounts/model"
)

type Endpoint struct {
	cfg   Config
	db    *DB
	limit Limiter
}

func New(db *DB, limit Limiter, cfg Config) *Endpoint {
	return &Endpoint{cfg: cfg, db: db, limit: limit}
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
	Create(ctx context.Context, u *model.User) error
}
