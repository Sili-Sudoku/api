package mockprovider

import (
	"context"
	"testing"

	"github.com/Sili-Sudoku/api/libs/validate"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Puzzle struct {
	T                  *testing.T
	AddReportFunc      func(ctx context.Context, id string, userID int64) (int, error)
	CreateFunc         func(ctx context.Context, p *puzzles.Puzzle) (created bool, err error)
	DeleteFunc         func(ctx context.Context, id string) error
	FindFunc           func(ctx context.Context, id string) (*puzzles.Puzzle, error)
	ListFunc           func(ctx context.Context, f puzzles.Filter) ([]*puzzles.Puzzle, error)
	PendingIDsFunc     func(ctx context.Context) ([]string, error)
	SetModerationFunc  func(ctx context.Context, id string, m puzzles.Moderation) error
	SetOwnerFunc       func(ctx context.Context, id string, ownerID int64) error
	SetRatingStatsFunc func(ctx context.Context, id string, r puzzles.RatingStats) error
	SetVerdictFunc     func(ctx context.Context, id string, status puzzles.Status, v validate.Verdict) error
}

func (m *Puzzle) AddReport(ctx context.Context, id string, userID int64) (int, error) {
	if m.AddReportFunc == nil {
		m.T.Fatal("unexpected call to Puzzle.AddReport")
	}

	return m.AddReportFunc(ctx, id, userID)
}

func (m *Puzzle) Create(ctx context.Context, p *puzzles.Puzzle) (bool, error) {
	if m.CreateFunc == nil {
		m.T.Fatal("unexpected call to Puzzle.Create")
	}

	return m.CreateFunc(ctx, p)
}

func (m *Puzzle) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc == nil {
		m.T.Fatal("unexpected call to Puzzle.Delete")
	}

	return m.DeleteFunc(ctx, id)
}

func (m *Puzzle) Find(ctx context.Context, id string) (*puzzles.Puzzle, error) {
	if m.FindFunc == nil {
		m.T.Fatal("unexpected call to Puzzle.Find")
	}

	return m.FindFunc(ctx, id)
}

func (m *Puzzle) List(ctx context.Context, f puzzles.Filter) ([]*puzzles.Puzzle, error) {
	if m.ListFunc == nil {
		m.T.Fatal("unexpected call to Puzzle.List")
	}

	return m.ListFunc(ctx, f)
}

func (m *Puzzle) PendingIDs(ctx context.Context) ([]string, error) {
	if m.PendingIDsFunc == nil {
		m.T.Fatal("unexpected call to Puzzle.PendingIDs")
	}

	return m.PendingIDsFunc(ctx)
}

func (m *Puzzle) SetModeration(ctx context.Context, id string, mod puzzles.Moderation) error {
	if m.SetModerationFunc == nil {
		m.T.Fatal("unexpected call to Puzzle.SetModeration")
	}

	return m.SetModerationFunc(ctx, id, mod)
}

func (m *Puzzle) SetOwner(ctx context.Context, id string, ownerID int64) error {
	if m.SetOwnerFunc == nil {
		m.T.Fatal("unexpected call to Puzzle.SetOwner")
	}

	return m.SetOwnerFunc(ctx, id, ownerID)
}

func (m *Puzzle) SetRatingStats(ctx context.Context, id string, r puzzles.RatingStats) error {
	if m.SetRatingStatsFunc == nil {
		m.T.Fatal("unexpected call to Puzzle.SetRatingStats")
	}

	return m.SetRatingStatsFunc(ctx, id, r)
}

func (m *Puzzle) SetVerdict(ctx context.Context, id string, status puzzles.Status, v validate.Verdict) error {
	if m.SetVerdictFunc == nil {
		m.T.Fatal("unexpected call to Puzzle.SetVerdict")
	}

	return m.SetVerdictFunc(ctx, id, status, v)
}

// Validator is what the publish endpoint starts in the background for a new puzzle.
type Validator struct {
	T            *testing.T
	ValidateFunc func(ctx context.Context, id string) error
}

func (m *Validator) Validate(ctx context.Context, id string) error {
	if m.ValidateFunc == nil {
		m.T.Fatal("unexpected call to Validator.Validate")
	}

	return m.ValidateFunc(ctx, id)
}
