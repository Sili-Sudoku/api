package mockprovider

import (
	"context"
	"testing"

	"github.com/Sili-Sudoku/api/services/community/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Rating struct {
	T         *testing.T
	FindFunc  func(ctx context.Context, puzzleID string, userID int64) (*model.Rating, error)
	PutFunc   func(ctx context.Context, puzzleID string, userID int64, r model.Rating) error
	StatsFunc func(ctx context.Context, puzzleID string) (puzzles.RatingStats, error)
}

func (m *Rating) Find(ctx context.Context, puzzleID string, userID int64) (*model.Rating, error) {
	if m.FindFunc == nil {
		m.T.Fatal("unexpected call to Rating.Find")
	}

	return m.FindFunc(ctx, puzzleID, userID)
}

func (m *Rating) Put(ctx context.Context, puzzleID string, userID int64, r model.Rating) error {
	if m.PutFunc == nil {
		m.T.Fatal("unexpected call to Rating.Put")
	}

	return m.PutFunc(ctx, puzzleID, userID, r)
}

func (m *Rating) Stats(ctx context.Context, puzzleID string) (puzzles.RatingStats, error) {
	if m.StatsFunc == nil {
		m.T.Fatal("unexpected call to Rating.Stats")
	}

	return m.StatsFunc(ctx, puzzleID)
}

type Comment struct {
	T                 *testing.T
	AddReportFunc     func(ctx context.Context, id, userID int64) (int, error)
	CreateFunc        func(ctx context.Context, c *model.Comment) error
	DeleteFunc        func(ctx context.Context, id int64) error
	FindFunc          func(ctx context.Context, id int64) (*model.Comment, error)
	ListFunc          func(ctx context.Context, puzzleID string, after *model.CommentCursor, limit int) ([]*model.Comment, error)
	SetModerationFunc func(ctx context.Context, id int64, m puzzles.Moderation) error
}

func (m *Comment) AddReport(ctx context.Context, id, userID int64) (int, error) {
	if m.AddReportFunc == nil {
		m.T.Fatal("unexpected call to Comment.AddReport")
	}

	return m.AddReportFunc(ctx, id, userID)
}

func (m *Comment) Create(ctx context.Context, c *model.Comment) error {
	if m.CreateFunc == nil {
		m.T.Fatal("unexpected call to Comment.Create")
	}

	return m.CreateFunc(ctx, c)
}

func (m *Comment) Delete(ctx context.Context, id int64) error {
	if m.DeleteFunc == nil {
		m.T.Fatal("unexpected call to Comment.Delete")
	}

	return m.DeleteFunc(ctx, id)
}

func (m *Comment) Find(ctx context.Context, id int64) (*model.Comment, error) {
	if m.FindFunc == nil {
		m.T.Fatal("unexpected call to Comment.Find")
	}

	return m.FindFunc(ctx, id)
}

func (m *Comment) List(ctx context.Context, puzzleID string, after *model.CommentCursor, limit int) ([]*model.Comment, error) {
	if m.ListFunc == nil {
		m.T.Fatal("unexpected call to Comment.List")
	}

	return m.ListFunc(ctx, puzzleID, after, limit)
}

func (m *Comment) SetModeration(ctx context.Context, id int64, mod puzzles.Moderation) error {
	if m.SetModerationFunc == nil {
		m.T.Fatal("unexpected call to Comment.SetModeration")
	}

	return m.SetModerationFunc(ctx, id, mod)
}

type Follow struct {
	T          *testing.T
	CountsFunc func(ctx context.Context, userID int64) (followers, following int, err error)
	CreateFunc func(ctx context.Context, followerID, creatorID int64) error
	DeleteFunc func(ctx context.Context, followerID, creatorID int64) error
	ExistsFunc func(ctx context.Context, followerID, creatorID int64) (bool, error)
	ListFunc   func(ctx context.Context, followerID int64, limit int) ([]int64, error)
}

func (m *Follow) Counts(ctx context.Context, userID int64) (int, int, error) {
	if m.CountsFunc == nil {
		m.T.Fatal("unexpected call to Follow.Counts")
	}

	return m.CountsFunc(ctx, userID)
}

func (m *Follow) Create(ctx context.Context, followerID, creatorID int64) error {
	if m.CreateFunc == nil {
		m.T.Fatal("unexpected call to Follow.Create")
	}

	return m.CreateFunc(ctx, followerID, creatorID)
}

func (m *Follow) Delete(ctx context.Context, followerID, creatorID int64) error {
	if m.DeleteFunc == nil {
		m.T.Fatal("unexpected call to Follow.Delete")
	}

	return m.DeleteFunc(ctx, followerID, creatorID)
}

func (m *Follow) Exists(ctx context.Context, followerID, creatorID int64) (bool, error) {
	if m.ExistsFunc == nil {
		m.T.Fatal("unexpected call to Follow.Exists")
	}

	return m.ExistsFunc(ctx, followerID, creatorID)
}

func (m *Follow) List(ctx context.Context, followerID int64, limit int) ([]int64, error) {
	if m.ListFunc == nil {
		m.T.Fatal("unexpected call to Follow.List")
	}

	return m.ListFunc(ctx, followerID, limit)
}

// SolveTimes is the SolveHistoryDB of the solve-stats endpoint.
type SolveTimes struct {
	T                *testing.T
	SolveSecondsFunc func(ctx context.Context, puzzleID string) ([]int, error)
}

func (m *SolveTimes) SolveSeconds(ctx context.Context, puzzleID string) ([]int, error) {
	if m.SolveSecondsFunc == nil {
		m.T.Fatal("unexpected call to SolveTimes.SolveSeconds")
	}

	return m.SolveSecondsFunc(ctx, puzzleID)
}
