package logout

import "context"

type Endpoint struct {
	cfg Config
	db  *DB
}

func New(db *DB, cfg Config) *Endpoint { return &Endpoint{cfg: cfg, db: db} }

type Config struct {
	InsecureCookie bool
}

type DB struct {
	Session SessionDB
}

type SessionDB interface {
	Delete(ctx context.Context, hash [32]byte) error
}
