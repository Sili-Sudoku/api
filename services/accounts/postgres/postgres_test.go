package postgres_test

import (
	"context"
	"errors"
	"github.com/Sili-Sudoku/api/libs/db"
	"testing"
	"time"

	"github.com/Sili-Sudoku/api/libs/dbtest"
	"github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/accounts/postgres"
	"github.com/Sili-Sudoku/api/services/accounts/session"
)

// These tests need TEST_DATABASE_URL and are skipped without it.
func TestUser(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.Tx(t)
	users := postgres.NewUser(tx)

	ann := &model.User{Username: "Ann_1", Email: "ann@example.com", PasswordHash: "h", CreatedAt: time.Now()}
	if err := users.Create(ctx, ann); err != nil || ann.ID == 0 {
		t.Fatalf("create: %v %+v", err, ann)
	}
	// The failed insert aborts the transaction, so it runs in a savepoint of its own.
	err := tx.InTx(ctx, func(sp db.DBTX) error {
		return postgres.NewUser(sp).Create(ctx, &model.User{Username: "ann_1", PasswordHash: "h", CreatedAt: time.Now()})
	})
	if !errors.Is(err, model.ErrUsernameTaken) {
		t.Fatalf("names are unique ignoring case: %v", err)
	}
	bob := &model.User{Username: "bob", PasswordHash: "h", CreatedAt: time.Now()}
	if err := users.Create(ctx, bob); err != nil {
		t.Fatal(err)
	}

	got, err := users.FindByName(ctx, "ANN_1")
	if err != nil || got == nil || got.ID != ann.ID || got.Email != "ann@example.com" || got.PasswordHash != "h" {
		t.Fatalf("find by name: %v %+v", err, got)
	}
	if got, err := users.FindByName(ctx, "nobody"); err != nil || got != nil {
		t.Fatalf("unknown user: %v %+v", err, got)
	}
	if got, _ := users.FindByName(ctx, "bob"); got.Email != "" {
		t.Fatalf("no email: %+v", got)
	}

	names, err := users.Usernames(ctx, []int64{ann.ID, bob.ID, 999999})
	if err != nil || len(names) != 2 || names[ann.ID] != "Ann_1" || names[bob.ID] != "bob" {
		t.Fatalf("usernames: %v %v", names, err)
	}

	if err := users.SetBio(ctx, ann.ID, "hello"); err != nil {
		t.Fatal(err)
	}
	if err := users.SetAdmin(ctx, ann.ID, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := users.FindByName(ctx, "ann_1"); got.Bio != "hello" || !got.IsAdmin {
		t.Fatalf("bio and admin: %+v", got)
	}
	if err := users.SetBio(ctx, 999999, "x"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("bio of nobody: %v", err)
	}
	if err := users.SetAdmin(ctx, 999999, true); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("admin of nobody: %v", err)
	}
}

func TestUser_Delete(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Tx(t)
	users, sessions := postgres.NewUser(pool), postgres.NewSession(pool)

	newUser := func(name string) *model.User {
		u := &model.User{Username: name, PasswordHash: "h", CreatedAt: time.Now()}
		if err := users.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		return u
	}
	puzzle := func(id string, owner int64) {
		if _, err := pool.Exec(ctx, `INSERT INTO puzzles (id, body, status, delete_token_hash, owner_id) VALUES ($1, '{}', 'published', '\x00', $2)`, id, owner); err != nil {
			t.Fatal(err)
		}
	}
	owner := func(id string) (n int64, found bool) {
		rows, err := pool.Query(ctx, `SELECT coalesce(owner_id, 0) FROM puzzles WHERE id = $1`, id)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			found = true
			if err := rows.Scan(&n); err != nil {
				t.Fatal(err)
			}
		}
		return n, found
	}

	ann, bob := newUser("ann"), newUser("bob")
	puzzle("ann-1", ann.ID)
	puzzle("bob-1", bob.ID)
	_, hash, _ := session.New()
	if err := sessions.Create(ctx, hash, ann.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	// Keeping the puzzles detaches them; the sessions go with the user.
	if err := users.Delete(ctx, ann.ID, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := users.FindByName(ctx, "ann"); got != nil {
		t.Fatalf("deleted user: %+v", got)
	}
	if n, found := owner("ann-1"); !found || n != 0 {
		t.Fatalf("kept puzzle: owner %d found %v", n, found)
	}
	if got, err := sessions.FindUser(ctx, hash, time.Now()); err != nil || got != nil {
		t.Fatalf("session of a deleted user: %v %+v", err, got)
	}
	if err := users.Delete(ctx, ann.ID, false); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}

	// Deleting the puzzles removes the rows, and only the user's own.
	if err := users.Delete(ctx, bob.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, found := owner("bob-1"); found {
		t.Fatal("puzzle of a deleted user must be gone")
	}
	if _, found := owner("ann-1"); !found {
		t.Fatal("anonymous puzzle must stay")
	}
}

func TestSession(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Tx(t)
	users, sessions := postgres.NewUser(pool), postgres.NewSession(pool)
	u := &model.User{Username: "ann", Email: "ann@example.com", PasswordHash: "h", CreatedAt: time.Now()}
	if err := users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	_, hash, _ := session.New()
	if err := sessions.Create(ctx, hash, u.ID, now.Add(session.TTL)); err != nil {
		t.Fatal(err)
	}
	got, err := sessions.FindUser(ctx, hash, now)
	if err != nil || got == nil || got.ID != u.ID || got.Email != "ann@example.com" {
		t.Fatalf("find user: %v %+v", err, got)
	}
	if got, err := sessions.FindUser(ctx, session.Hash("unknown"), now); err != nil || got != nil {
		t.Fatalf("unknown session: %v %+v", err, got)
	}

	// A session ends at its expiry.
	if got, err := sessions.FindUser(ctx, hash, now.Add(session.TTL+time.Second)); err != nil || got != nil {
		t.Fatalf("expired session: %v %+v", err, got)
	}

	if err := sessions.Delete(ctx, hash); err != nil {
		t.Fatal(err)
	}
	if got, err := sessions.FindUser(ctx, hash, now); err != nil || got != nil {
		t.Fatalf("logged-out session: %v %+v", err, got)
	}
}
