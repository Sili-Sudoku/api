package postgres

import (
	"context"

	"github.com/Sili-Sudoku/api/libs/db"
)

type Follow struct {
	db db.DBTX
}

func NewFollow(conn db.DBTX) *Follow { return &Follow{db: conn} }

// Create is idempotent.
func (this *Follow) Create(ctx context.Context, followerID, creatorID int64) error {
	_, err := this.db.Exec(ctx,
		`INSERT INTO follows (follower_id, creator_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, followerID, creatorID)
	return err
}

// Delete is idempotent.
func (this *Follow) Delete(ctx context.Context, followerID, creatorID int64) error {
	_, err := this.db.Exec(ctx, `DELETE FROM follows WHERE follower_id = $1 AND creator_id = $2`, followerID, creatorID)
	return err
}

func (this *Follow) Exists(ctx context.Context, followerID, creatorID int64) (bool, error) {
	var ok bool
	err := this.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM follows WHERE follower_id = $1 AND creator_id = $2)`, followerID, creatorID).Scan(&ok)

	return ok, err
}

// Counts returns how many follow the user and how many the user follows.
func (this *Follow) Counts(ctx context.Context, userID int64) (followers, following int, err error) {
	err = this.db.QueryRow(ctx,
		`SELECT (SELECT count(*) FROM follows WHERE creator_id = $1), (SELECT count(*) FROM follows WHERE follower_id = $1)`,
		userID).Scan(&followers, &following)

	return
}

// List returns the creators a user follows, newest follow first, at most limit.
func (this *Follow) List(ctx context.Context, followerID int64, limit int) ([]int64, error) {
	rows, err := this.db.Query(ctx,
		`SELECT creator_id FROM follows WHERE follower_id = $1 ORDER BY created_at DESC, creator_id DESC LIMIT $2`,
		followerID, limit)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		out = append(out, id)
	}

	return out, rows.Err()
}
