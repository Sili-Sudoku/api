package postgres

import (
	"context"

	"github.com/Sili-Sudoku/api/libs/db"
	"github.com/Sili-Sudoku/api/services/history/model"
)

type SolveHistory struct {
	db db.DBTX
}

func NewSolveHistory(conn db.DBTX) *SolveHistory { return &SolveHistory{db: conn} }

// Upsert merges e into the stored entry: the lower time wins and brings its date; otherwise an unknown date is filled in.
// It returns model.ErrFull instead of adding entry number MaxEntries+1.
func (this *SolveHistory) Upsert(ctx context.Context, userID int64, e model.Entry) (model.Entry, error) {
	tx, err := this.db.Begin(ctx)
	if err != nil {
		return model.Entry{}, err
	}

	defer func() { _ = tx.Rollback(ctx) }()
	// Serialize the writes of one user, so the count check below cannot be raced past.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, userID); err != nil {
		return model.Entry{}, err
	}

	var n int
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT count(*), bool_or(puzzle_id = $2) IS TRUE FROM solve_history WHERE user_id = $1`,
		userID, e.PuzzleID).Scan(&n, &exists); err != nil {
		return model.Entry{}, err
	}

	if !exists && n >= model.MaxEntries {
		return model.Entry{}, model.ErrFull
	}
	// The lower time wins and takes its date; otherwise an unknown date is filled in.
	var out model.Entry
	err = tx.QueryRow(ctx,
		`INSERT INTO solve_history AS h (user_id, puzzle_id, solved_at, seconds)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, puzzle_id) DO UPDATE SET
		     solved_at = CASE WHEN excluded.seconds IS NOT NULL AND (h.seconds IS NULL OR excluded.seconds < h.seconds)
		                      THEN excluded.solved_at ELSE coalesce(h.solved_at, excluded.solved_at) END,
		     seconds = CASE WHEN excluded.seconds IS NOT NULL AND (h.seconds IS NULL OR excluded.seconds < h.seconds)
		                    THEN excluded.seconds ELSE h.seconds END,
		     updated_at = now()
		 RETURNING puzzle_id, solved_at, seconds, updated_at`,
		userID, e.PuzzleID, e.SolvedAt, e.Seconds).Scan(&out.PuzzleID, &out.SolvedAt, &out.Seconds, &out.UpdatedAt)
	if err != nil {
		return model.Entry{}, err
	}

	return out, tx.Commit(ctx)
}

func (this *SolveHistory) List(ctx context.Context, userID int64, after *model.Cursor, limit int) ([]model.Entry, error) {
	var afterT any
	afterID := ""
	if after != nil {
		afterT, afterID = after.UpdatedAt, after.PuzzleID
	}
	rows, err := this.db.Query(ctx,
		`SELECT puzzle_id, solved_at, seconds, updated_at FROM solve_history
		  WHERE user_id = $1 AND ($2::timestamptz IS NULL OR (updated_at, puzzle_id COLLATE "C") < ($2::timestamptz, $3::text))
		  ORDER BY updated_at DESC, puzzle_id COLLATE "C" DESC LIMIT $4`,
		userID, afterT, afterID, limit)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var out []model.Entry
	for rows.Next() {
		var e model.Entry
		if err := rows.Scan(&e.PuzzleID, &e.SolvedAt, &e.Seconds, &e.UpdatedAt); err != nil {
			return nil, err
		}

		out = append(out, e)
	}

	return out, rows.Err()
}

func (this *SolveHistory) DeleteAll(ctx context.Context, userID int64) error {
	_, err := this.db.Exec(ctx, `DELETE FROM solve_history WHERE user_id = $1`, userID)
	return err
}

// SolveSeconds returns the best times of all users for one puzzle.
func (this *SolveHistory) SolveSeconds(ctx context.Context, puzzleID string) ([]int, error) {
	rows, err := this.db.Query(ctx,
		`SELECT seconds FROM solve_history WHERE puzzle_id = $1 AND seconds IS NOT NULL LIMIT 10000`, puzzleID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var out []int
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}

		out = append(out, n)
	}

	return out, rows.Err()
}
