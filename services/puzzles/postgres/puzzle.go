package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Sili-Sudoku/api/libs/db"
	"github.com/Sili-Sudoku/api/libs/validate"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Puzzle struct {
	db db.DBTX
}

func NewPuzzle(conn db.DBTX) *Puzzle { return &Puzzle{db: conn} }

const cols = `id, body, status, verdict, delete_token_hash, report_count, moderation, coalesce(owner_id, 0), created_at,
	title, author, source, difficulty, tags, variants, rating_count, rating_sum, diff_count, diff_sum`

func scan(row db.Row) (*model.Puzzle, error) {
	var (
		p       model.Puzzle
		status  string
		mod     string
		verdict []byte
		hash    []byte
	)
	err := row.Scan(&p.ID, &p.Body, &status, &verdict, &hash, &p.Reports, &mod, &p.OwnerID, &p.CreatedAt,
		&p.Title, &p.Author, &p.Source, &p.Difficulty, &p.Tags, &p.Variants,
		&p.Ratings.StarsCount, &p.Ratings.StarsSum, &p.Ratings.DifficultyCount, &p.Ratings.DifficultySum)
	if errors.Is(err, db.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	p.Status, p.Moderation = model.Status(status), model.Moderation(mod)
	copy(p.TokenHash[:], hash)
	if verdict != nil {
		var v validate.Verdict
		if err := json.Unmarshal(verdict, &v); err != nil {
			return nil, err
		}

		p.Verdict = &v
	}

	return &p, nil
}

func (this *Puzzle) Create(ctx context.Context, p *model.Puzzle) (bool, error) {
	tag, err := this.db.Exec(ctx,
		`INSERT INTO puzzles (id, body, status, delete_token_hash, owner_id, created_at, title, author, source, difficulty, tags, variants)
		 VALUES ($1, $2, $3, $4, nullif($5, 0), $6, $7, $8, $9, $10, $11, $12) ON CONFLICT (id) DO NOTHING`,
		p.ID, p.Body, string(p.Status), p.TokenHash[:], p.OwnerID, p.CreatedAt,
		p.Title, p.Author, p.Source, p.Difficulty, p.Tags, p.Variants)
	if err != nil {
		return false, err
	}

	return tag.RowsAffected() == 1, nil
}

func (this *Puzzle) Find(ctx context.Context, id string) (*model.Puzzle, error) {
	p, err := scan(this.db.QueryRow(ctx, `SELECT `+cols+` FROM puzzles WHERE id = $1`, id))
	if errors.Is(err, model.ErrNotFound) {
		return nil, nil
	}

	return p, err
}

func (this *Puzzle) SetVerdict(ctx context.Context, id string, status model.Status, v validate.Verdict) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return affected(this.db.Exec(ctx, `UPDATE puzzles SET status = $2, verdict = $3 WHERE id = $1`, id, string(status), raw))
}

// AddReport inserts the report and bumps the counter in one statement, so a repeat report changes nothing.
func (this *Puzzle) AddReport(ctx context.Context, id string, userID int64) (int, error) {
	var n int
	err := this.db.QueryRow(ctx,
		`WITH ins AS (
		     INSERT INTO reports (kind, target, user_id)
		     SELECT 'puzzle', id, $2 FROM puzzles WHERE id = $1
		     ON CONFLICT DO NOTHING RETURNING 1)
		 UPDATE puzzles SET report_count = report_count + (SELECT count(*) FROM ins)
		  WHERE id = $1 RETURNING report_count`, id, userID).Scan(&n)
	if errors.Is(err, db.ErrNoRows) {
		return 0, model.ErrNotFound
	}

	return n, err
}

func (this *Puzzle) Delete(ctx context.Context, id string) error {
	return affected(this.db.Exec(ctx, `DELETE FROM puzzles WHERE id = $1`, id))
}

func (this *Puzzle) SetOwner(ctx context.Context, id string, ownerID int64) error {
	return affected(this.db.Exec(ctx, `UPDATE puzzles SET owner_id = $2 WHERE id = $1`, id, ownerID))
}

func (this *Puzzle) SetModeration(ctx context.Context, id string, m model.Moderation) error {
	return affected(this.db.Exec(ctx, `UPDATE puzzles SET moderation = $2 WHERE id = $1`, id, string(m)))
}

func (this *Puzzle) SetRatingStats(ctx context.Context, id string, r model.RatingStats) error {
	return affected(this.db.Exec(ctx,
		`UPDATE puzzles SET rating_count = $2, rating_sum = $3, diff_count = $4, diff_sum = $5 WHERE id = $1`,
		id, r.StarsCount, r.StarsSum, r.DifficultyCount, r.DifficultySum))
}

func (this *Puzzle) List(ctx context.Context, f model.Filter) ([]*model.Puzzle, error) {
	var afterT any // NULL when there is no cursor
	afterID := ""
	if f.After != nil {
		afterT, afterID = f.After.CreatedAt, f.After.ID
	}
	rows, err := this.db.Query(ctx,
		`SELECT `+cols+` FROM puzzles
		  WHERE status = 'published' AND moderation <> 'hidden' AND (moderation = 'cleared' OR report_count < $8)
		    AND ($1::bigint[] IS NULL OR owner_id = ANY($1))
		    AND ($2::text = '' OR $2 = ANY(tags))
		    AND ($3::text = '' OR $3 = ANY(variants))
		    AND ($4::text = '' OR difficulty = $4)
		    AND ($5::timestamptz IS NULL OR (created_at, id COLLATE "C") < ($5::timestamptz, $6::text))
		  ORDER BY CASE WHEN $9 THEN (rating_sum + 6) * 1000 / (rating_count + 2) END DESC NULLS LAST,
		           created_at DESC, id COLLATE "C" DESC
		  LIMIT $7 OFFSET $10`,
		f.OwnerIDs, f.Tag, f.Variant, f.Difficulty, afterT, afterID, f.Limit, model.ReportLimit, f.Top, f.Offset)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var out []*model.Puzzle
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, p)
	}

	return out, rows.Err()
}

func (this *Puzzle) PendingIDs(ctx context.Context) ([]string, error) {
	rows, err := this.db.Query(ctx, `SELECT id FROM puzzles WHERE status = 'pending' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, rows.Err()
}

func affected(tag interface{ RowsAffected() int64 }, err error) error {
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}

	return nil
}
