package postgres

import (
	"context"
	"errors"
	"strconv"

	"github.com/Sili-Sudoku/api/libs/db"
	"github.com/Sili-Sudoku/api/services/community/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Comment struct {
	db db.DBTX
}

func NewComment(conn db.DBTX) *Comment { return &Comment{db: conn} }

const commentCols = `id, puzzle_id, user_id, body, report_count, moderation, created_at`

func scanComment(row db.Row) (*model.Comment, error) {
	var c model.Comment
	var mod string
	if err := row.Scan(&c.ID, &c.PuzzleID, &c.UserID, &c.Body, &c.Reports, &mod, &c.CreatedAt); err != nil {
		return nil, err
	}

	c.Moderation = puzzles.Moderation(mod)

	return &c, nil
}

// Create sets the ID and the CreatedAt of c.
func (this *Comment) Create(ctx context.Context, c *model.Comment) error {
	return this.db.QueryRow(ctx,
		`INSERT INTO comments (puzzle_id, user_id, body) VALUES ($1, $2, $3) RETURNING id, created_at`,
		c.PuzzleID, c.UserID, c.Body).Scan(&c.ID, &c.CreatedAt)
}

func (this *Comment) Find(ctx context.Context, id int64) (*model.Comment, error) {
	c, err := scanComment(this.db.QueryRow(ctx, `SELECT `+commentCols+` FROM comments WHERE id = $1`, id))
	if errors.Is(err, db.ErrNoRows) {
		return nil, nil
	}

	return c, err
}

// Delete removes the comment and its reports in one transaction.
func (this *Comment) Delete(ctx context.Context, id int64) error {
	return this.db.InTx(ctx, func(tx db.DBTX) error {
		tag, err := tx.Exec(ctx, `DELETE FROM comments WHERE id = $1`, id)
		if err != nil {
			return err
		}

		if tag.RowsAffected() == 0 {
			return model.ErrNotFound
		}
		_, err = tx.Exec(ctx, `DELETE FROM reports WHERE kind = 'comment' AND target = $1`, strconv.FormatInt(id, 10))

		return err
	})
}

// List returns the visible comments of the puzzle, newest first, at most limit.
func (this *Comment) List(ctx context.Context, puzzleID string, after *model.CommentCursor, limit int) ([]*model.Comment, error) {
	var afterT any
	var afterID int64
	if after != nil {
		afterT, afterID = after.CreatedAt, after.ID
	}
	rows, err := this.db.Query(ctx,
		`SELECT `+commentCols+` FROM comments
		  WHERE puzzle_id = $1 AND moderation <> 'hidden' AND (moderation = 'cleared' OR report_count < $2)
		    AND ($3::timestamptz IS NULL OR (created_at, id) < ($3::timestamptz, $4::bigint))
		  ORDER BY created_at DESC, id DESC LIMIT $5`,
		puzzleID, model.ReportLimit, afterT, afterID, limit)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var out []*model.Comment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, c)
	}

	return out, rows.Err()
}

// AddReport records one report per user and returns the new total.
func (this *Comment) AddReport(ctx context.Context, id, userID int64) (int, error) {
	var n int
	err := this.db.QueryRow(ctx,
		`WITH ins AS (
		     INSERT INTO reports (kind, target, user_id)
		     SELECT 'comment', id::text, $2 FROM comments WHERE id = $1
		     ON CONFLICT DO NOTHING RETURNING 1)
		 UPDATE comments SET report_count = report_count + (SELECT count(*) FROM ins)
		  WHERE id = $1 RETURNING report_count`, id, userID).Scan(&n)
	if errors.Is(err, db.ErrNoRows) {
		return 0, model.ErrNotFound
	}

	return n, err
}

func (this *Comment) SetModeration(ctx context.Context, id int64, m puzzles.Moderation) error {
	tag, err := this.db.Exec(ctx, `UPDATE comments SET moderation = $2 WHERE id = $1`, id, string(m))
	if err == nil && tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}

	return err
}
