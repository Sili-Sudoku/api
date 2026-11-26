package postgres

import (
	"context"
	"errors"

	"github.com/Sili-Sudoku/api/libs/db"
	"github.com/Sili-Sudoku/api/services/accounts/model"
)

type User struct {
	db db.DBTX
}

func NewUser(conn db.DBTX) *User { return &User{db: conn} }

const userCols = `id, username, coalesce(email, ''), password_hash, bio, is_admin, created_at`

// scanUser maps no row to nil, nil, as every Find does.
func scanUser(row db.Row) (*model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Bio, &u.IsAdmin, &u.CreatedAt)
	if errors.Is(err, db.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &u, nil
}

// Create sets u.ID. It returns model.ErrUsernameTaken when the name exists, ignoring case.
func (this *User) Create(ctx context.Context, u *model.User) error {
	err := this.db.QueryRow(ctx,
		`INSERT INTO users (username, email, password_hash, created_at) VALUES ($1, nullif($2, ''), $3, $4) RETURNING id`,
		u.Username, u.Email, u.PasswordHash, u.CreatedAt).Scan(&u.ID)
	if errors.Is(err, db.ErrUniqueViolation) {
		return model.ErrUsernameTaken
	}

	return err
}

func (this *User) FindByName(ctx context.Context, name string) (*model.User, error) {
	return scanUser(this.db.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE lower(username) = lower($1)`, name))
}

// Usernames maps user ids to usernames. Unknown ids are left out.
func (this *User) Usernames(ctx context.Context, ids []int64) (map[int64]string, error) {
	rows, err := this.db.Query(ctx, `SELECT id, username FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}

		out[id] = name
	}

	return out, rows.Err()
}

func (this *User) SetBio(ctx context.Context, id int64, bio string) error {
	return found(this.db.Exec(ctx, `UPDATE users SET bio = $2 WHERE id = $1`, id, bio))
}

func (this *User) SetAdmin(ctx context.Context, id int64, admin bool) error {
	return found(this.db.Exec(ctx, `UPDATE users SET is_admin = $2 WHERE id = $1`, id, admin))
}

// Delete removes the user and the sessions with it. The puzzles of the user are deleted or, when deletePuzzles is false,
// kept without an owner. All of it is one transaction, so a failure leaves the account whole.
func (this *User) Delete(ctx context.Context, id int64, deletePuzzles bool) error {
	puzzles := `UPDATE puzzles SET owner_id = NULL WHERE owner_id = $1`
	if deletePuzzles {
		puzzles = `DELETE FROM puzzles WHERE owner_id = $1`
	}

	return this.db.InTx(ctx, func(tx db.DBTX) error {
		if _, err := tx.Exec(ctx, puzzles, id); err != nil {
			return err
		}

		return found(tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)) // sessions cascade
	})
}

func found(tag interface{ RowsAffected() int64 }, err error) error {
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}

	return nil
}
