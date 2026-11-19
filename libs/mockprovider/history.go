package mockprovider

import (
	"context"
	"testing"

	"github.com/Sili-Sudoku/api/services/history/model"
)

// Auth fails the test when a method without a Func is called; the other mocks work the same way.
type Auth struct {
	T             *testing.T
	GetUserIDFunc func(ctx context.Context) (int64, error)
}

func (m *Auth) GetUserID(ctx context.Context) (int64, error) {
	if m.GetUserIDFunc == nil {
		m.T.Fatal("unexpected call to Auth.GetUserID")
	}

	return m.GetUserIDFunc(ctx)
}

type SolveHistory struct {
	T             *testing.T
	DeleteAllFunc func(ctx context.Context, userID int64) error
	ListFunc      func(ctx context.Context, userID int64, after *model.Cursor, limit int) ([]model.Entry, error)
	UpsertFunc    func(ctx context.Context, userID int64, e model.Entry) (model.Entry, error)
}

func (m *SolveHistory) DeleteAll(ctx context.Context, userID int64) error {
	if m.DeleteAllFunc == nil {
		m.T.Fatal("unexpected call to SolveHistory.DeleteAll")
	}

	return m.DeleteAllFunc(ctx, userID)
}

func (m *SolveHistory) List(ctx context.Context, userID int64, after *model.Cursor, limit int) ([]model.Entry, error) {
	if m.ListFunc == nil {
		m.T.Fatal("unexpected call to SolveHistory.List")
	}

	return m.ListFunc(ctx, userID, after, limit)
}

func (m *SolveHistory) Upsert(ctx context.Context, userID int64, e model.Entry) (model.Entry, error) {
	if m.UpsertFunc == nil {
		m.T.Fatal("unexpected call to SolveHistory.Upsert")
	}

	return m.UpsertFunc(ctx, userID, e)
}
