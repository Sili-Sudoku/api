package mockprovider

import (
	"context"
	"testing"

	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
)

// UserAuth is the Auth of an endpoint that needs the whole signed-in user and answers 401 without one.
type UserAuth struct {
	T           *testing.T
	GetUserFunc func(ctx context.Context) (*accounts.User, error)
}

func (m *UserAuth) GetUser(ctx context.Context) (*accounts.User, error) {
	if m.GetUserFunc == nil {
		m.T.Fatal("unexpected call to UserAuth.GetUser")
	}

	return m.GetUserFunc(ctx)
}

// OptionalAuth is the Auth of an endpoint that also serves signed-out callers: nil means nobody is signed in.
type OptionalAuth struct {
	T            *testing.T
	FindUserFunc func(ctx context.Context) *accounts.User
}

func (m *OptionalAuth) FindUser(ctx context.Context) *accounts.User {
	if m.FindUserFunc == nil {
		m.T.Fatal("unexpected call to OptionalAuth.FindUser")
	}

	return m.FindUserFunc(ctx)
}

type Limiter struct {
	T         *testing.T
	AllowFunc func(key string) bool
}

func (m *Limiter) Allow(key string) bool {
	if m.AllowFunc == nil {
		m.T.Fatal("unexpected call to Limiter.Allow")
	}

	return m.AllowFunc(key)
}
