package deleteme

import (
	"context"

	"github.com/Sili-Sudoku/api/services/accounts/model"
)

type Endpoint struct {
	auth  Auth
	cfg   Config
	db    *DB
	limit Limiter
}

func New(auth Auth, db *DB, limit Limiter, cfg Config) *Endpoint {
	return &Endpoint{auth: auth, cfg: cfg, db: db, limit: limit}
}

type Config struct {
	InsecureCookie bool
}

type Auth interface {
	GetUser(ctx context.Context) (*model.User, error)
}

type DB struct {
	User UserDB
}

// Limiter is the log-in limiter by username: a stolen session must not be able to guess the password.
type Limiter interface {
	Allow(key string) bool
}

type UserDB interface {
	Delete(ctx context.Context, id int64, deletePuzzles bool) error
}
