package postgres

import (
	"context"
	"time"

	"github.com/Sili-Sudoku/api/libs/db"
	"github.com/Sili-Sudoku/api/services/accounts/model"
)

type Session struct {
	db db.DBTX
}

func NewSession(conn db.DBTX) *Session { return &Session{db: conn} }

func (this *Session) Create(ctx context.Context, hash [32]byte, userID int64, expires time.Time) error {
	_, err := this.db.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`, hash[:], userID, expires)
	return err
}

// FindUser returns the user of a session that has not expired at now, or nil.
func (this *Session) FindUser(ctx context.Context, hash [32]byte, now time.Time) (*model.User, error) {
	return scanUser(this.db.QueryRow(ctx,
		`SELECT u.id, u.username, coalesce(u.email, ''), u.password_hash, u.bio, u.is_admin, u.created_at
		   FROM sessions s JOIN users u ON u.id = s.user_id
		  WHERE s.token_hash = $1 AND s.expires_at > $2`, hash[:], now))
}

func (this *Session) Delete(ctx context.Context, hash [32]byte) error {
	_, err := this.db.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hash[:])
	return err
}
