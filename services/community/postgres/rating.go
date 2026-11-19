package postgres

import (
	"context"
	"errors"

	"github.com/Sili-Sudoku/api/libs/db"
	"github.com/Sili-Sudoku/api/services/community/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
	"github.com/jackc/pgx/v5"
)

type Rating struct {
	db db.DBTX
}

func NewRating(conn db.DBTX) *Rating { return &Rating{db: conn} }

// Put replaces the user's rating of the puzzle. A rating with both fields at 0 removes it.
func (this *Rating) Put(ctx context.Context, puzzleID string, userID int64, r model.Rating) error {
	if r == (model.Rating{}) {
		_, err := this.db.Exec(ctx, `DELETE FROM ratings WHERE puzzle_id = $1 AND user_id = $2`, puzzleID, userID)
		return err
	}
	_, err := this.db.Exec(ctx,
		`INSERT INTO ratings (puzzle_id, user_id, stars, difficulty) VALUES ($1, $2, nullif($3, 0), nullif($4, 0))
		 ON CONFLICT (puzzle_id, user_id) DO UPDATE SET stars = excluded.stars, difficulty = excluded.difficulty`,
		puzzleID, userID, r.Stars, r.Difficulty)

	return err
}

func (this *Rating) Find(ctx context.Context, puzzleID string, userID int64) (*model.Rating, error) {
	var r model.Rating
	err := this.db.QueryRow(ctx,
		`SELECT coalesce(stars, 0), coalesce(difficulty, 0) FROM ratings WHERE puzzle_id = $1 AND user_id = $2`,
		puzzleID, userID).Scan(&r.Stars, &r.Difficulty)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &r, nil
}

// Stats sums all ratings of the puzzle.
func (this *Rating) Stats(ctx context.Context, puzzleID string) (puzzles.RatingStats, error) {
	var st puzzles.RatingStats
	err := this.db.QueryRow(ctx,
		`SELECT count(stars), coalesce(sum(stars), 0), count(difficulty), coalesce(sum(difficulty), 0)
		   FROM ratings WHERE puzzle_id = $1`, puzzleID).
		Scan(&st.StarsCount, &st.StarsSum, &st.DifficultyCount, &st.DifficultySum)

	return st, err
}
