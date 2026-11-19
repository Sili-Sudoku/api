package mockprovider

import (
	"context"
	"testing"
	"time"

	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
)

type User struct {
	T              *testing.T
	CreateFunc     func(ctx context.Context, u *accounts.User) error
	DeleteFunc     func(ctx context.Context, id int64, deletePuzzles bool) error
	FindByNameFunc func(ctx context.Context, name string) (*accounts.User, error)
	SetAdminFunc   func(ctx context.Context, id int64, admin bool) error
	SetBioFunc     func(ctx context.Context, id int64, bio string) error
	UsernamesFunc  func(ctx context.Context, ids []int64) (map[int64]string, error)
}

func (m *User) Create(ctx context.Context, u *accounts.User) error {
	if m.CreateFunc == nil {
		m.T.Fatal("unexpected call to User.Create")
	}

	return m.CreateFunc(ctx, u)
}

func (m *User) Delete(ctx context.Context, id int64, deletePuzzles bool) error {
	if m.DeleteFunc == nil {
		m.T.Fatal("unexpected call to User.Delete")
	}

	return m.DeleteFunc(ctx, id, deletePuzzles)
}

func (m *User) FindByName(ctx context.Context, name string) (*accounts.User, error) {
	if m.FindByNameFunc == nil {
		m.T.Fatal("unexpected call to User.FindByName")
	}

	return m.FindByNameFunc(ctx, name)
}

func (m *User) SetAdmin(ctx context.Context, id int64, admin bool) error {
	if m.SetAdminFunc == nil {
		m.T.Fatal("unexpected call to User.SetAdmin")
	}

	return m.SetAdminFunc(ctx, id, admin)
}

func (m *User) SetBio(ctx context.Context, id int64, bio string) error {
	if m.SetBioFunc == nil {
		m.T.Fatal("unexpected call to User.SetBio")
	}

	return m.SetBioFunc(ctx, id, bio)
}

func (m *User) Usernames(ctx context.Context, ids []int64) (map[int64]string, error) {
	if m.UsernamesFunc == nil {
		m.T.Fatal("unexpected call to User.Usernames")
	}

	return m.UsernamesFunc(ctx, ids)
}

type Session struct {
	T            *testing.T
	CreateFunc   func(ctx context.Context, hash [32]byte, userID int64, expires time.Time) error
	DeleteFunc   func(ctx context.Context, hash [32]byte) error
	FindUserFunc func(ctx context.Context, hash [32]byte, now time.Time) (*accounts.User, error)
}

func (m *Session) Create(ctx context.Context, hash [32]byte, userID int64, expires time.Time) error {
	if m.CreateFunc == nil {
		m.T.Fatal("unexpected call to Session.Create")
	}

	return m.CreateFunc(ctx, hash, userID, expires)
}

func (m *Session) Delete(ctx context.Context, hash [32]byte) error {
	if m.DeleteFunc == nil {
		m.T.Fatal("unexpected call to Session.Delete")
	}

	return m.DeleteFunc(ctx, hash)
}

func (m *Session) FindUser(ctx context.Context, hash [32]byte, now time.Time) (*accounts.User, error) {
	if m.FindUserFunc == nil {
		m.T.Fatal("unexpected call to Session.FindUser")
	}

	return m.FindUserFunc(ctx, hash, now)
}
