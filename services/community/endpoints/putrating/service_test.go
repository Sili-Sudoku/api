package putrating

import (
	"context"
	"errors"
	"net/http"
	"testing"

	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/community/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

func okAuth(t *testing.T) *mp.UserAuth {
	return &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*accounts.User, error) { return &accounts.User{ID: 7}, nil }}
}

func TestNew(t *testing.T) {
	auth, db := &mp.UserAuth{T: t}, &DB{}
	if ep := New(auth, db); ep.auth != auth || ep.db != db {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_BindPath(t *testing.T) {
	mp.RunBindPathTests(t, []mp.BindPathTest{
		{Name: "id", PathValues: map[string]string{"id": "p1"}, Input: &EndpointReq{}, Want: &EndpointReq{PuzzleID: "p1"}},
	})
}

func TestEndpointReq_Validate(t *testing.T) {
	mp.RunValidateTests(t, []mp.ValidateTest{
		{Name: "stars 6", Input: &EndpointReq{Stars: 6}, WantErr: true},
		{Name: "stars -1", Input: &EndpointReq{Stars: -1}, WantErr: true},
		{Name: "difficulty 6", Input: &EndpointReq{Stars: 3, Difficulty: 6}, WantErr: true},
		{Name: "both given", Input: &EndpointReq{Stars: 5, Difficulty: 1}},
		{Name: "only stars", Input: &EndpointReq{Stars: 3}},
		{Name: "both zero removes the rating", Input: &EndpointReq{}},
	})
}

func TestEndpoint_Execute(t *testing.T) {
	published := &puzzles.Puzzle{Status: puzzles.Published, OwnerID: 99}
	stats := puzzles.RatingStats{StarsCount: 2, StarsSum: 8, DifficultyCount: 1, DifficultySum: 4}
	puzzleOf := func(p *puzzles.Puzzle, err error, set func(ctx context.Context, id string, r puzzles.RatingStats) error) *mp.Puzzle {
		return &mp.Puzzle{T: t, FindFunc: func(ctx context.Context, id string) (*puzzles.Puzzle, error) { return p, err }, SetRatingStatsFunc: set}
	}
	// ratings builds the rating mock; a nil func is unexpected and fails the test.
	ratings := func(put func(ctx context.Context, puzzleID string, userID int64, r model.Rating) error,
		stats func(ctx context.Context, puzzleID string) (puzzles.RatingStats, error),
		find func(ctx context.Context, puzzleID string, userID int64) (*model.Rating, error)) *mp.Rating {
		return &mp.Rating{T: t, PutFunc: put, StatsFunc: stats, FindFunc: find}
	}
	okPut := func(ctx context.Context, puzzleID string, userID int64, r model.Rating) error { return nil }
	okStats := func(ctx context.Context, puzzleID string) (puzzles.RatingStats, error) { return stats, nil }
	okSet := func(ctx context.Context, id string, r puzzles.RatingStats) error { return nil }

	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name: "auth.GetUser_error",
			Endpoint: &Endpoint{
				auth: &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*accounts.User, error) { return nil, errors.New("authErr") }},
				db:   &DB{},
			},
			Input:   &EndpointReq{},
			WantErr: true,
		},
		{
			Name:     "db.Puzzle.Find_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{Puzzle: puzzleOf(nil, errors.New("dbErr"), nil)}},
			Input:    &EndpointReq{PuzzleID: "p1"},
			WantErr:  true,
		},
		{
			Name:       "db.Puzzle.Find_missing",
			Endpoint:   &Endpoint{auth: okAuth(t), db: &DB{Puzzle: puzzleOf(nil, nil, nil)}},
			Input:      &EndpointReq{PuzzleID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name:       "puzzle_not_published",
			Endpoint:   &Endpoint{auth: okAuth(t), db: &DB{Puzzle: puzzleOf(&puzzles.Puzzle{Status: puzzles.Pending}, nil, nil)}},
			Input:      &EndpointReq{PuzzleID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name:       "own_puzzle",
			Endpoint:   &Endpoint{auth: okAuth(t), db: &DB{Puzzle: puzzleOf(&puzzles.Puzzle{Status: puzzles.Published, OwnerID: 7}, nil, nil)}},
			Input:      &EndpointReq{PuzzleID: "p1", Stars: 5},
			WantErr:    true,
			WantStatus: http.StatusForbidden,
			WantCode:   "own_puzzle",
		},
		{
			Name: "db.Rating.Put_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{
				Puzzle: puzzleOf(published, nil, nil),
				Rating: ratings(func(ctx context.Context, puzzleID string, userID int64, r model.Rating) error {
					return errors.New("dbErr")
				}, nil, nil),
			}},
			Input:   &EndpointReq{PuzzleID: "p1", Stars: 5},
			WantErr: true,
		},
		{
			Name: "db.Rating.Stats_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{
				Puzzle: puzzleOf(published, nil, nil),
				Rating: ratings(okPut, func(ctx context.Context, puzzleID string) (puzzles.RatingStats, error) {
					return puzzles.RatingStats{}, errors.New("dbErr")
				}, nil),
			}},
			Input:   &EndpointReq{PuzzleID: "p1", Stars: 5},
			WantErr: true,
		},
		{
			Name: "db.Puzzle.SetRatingStats_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{
				Puzzle: puzzleOf(published, nil, func(ctx context.Context, id string, r puzzles.RatingStats) error { return errors.New("dbErr") }),
				Rating: ratings(okPut, okStats, nil),
			}},
			Input:   &EndpointReq{PuzzleID: "p1", Stars: 5},
			WantErr: true,
		},
		{
			Name: "db.Rating.Find_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{
				Puzzle: puzzleOf(published, nil, okSet),
				Rating: ratings(okPut, okStats, func(ctx context.Context, puzzleID string, userID int64) (*model.Rating, error) {
					return nil, errors.New("dbErr")
				}),
			}},
			Input:   &EndpointReq{PuzzleID: "p1", Stars: 5},
			WantErr: true,
		},
		{
			Name: "success",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{
				Puzzle: puzzleOf(published, nil, func(ctx context.Context, id string, r puzzles.RatingStats) error {
					if id != "p1" || r != stats {
						t.Errorf("SetRatingStats(%q, %+v)", id, r)
					}
					return nil
				}),
				Rating: ratings(func(ctx context.Context, puzzleID string, userID int64, r model.Rating) error {
					if puzzleID != "p1" || userID != 7 || r != (model.Rating{Stars: 5, Difficulty: 4}) {
						t.Errorf("Put(%q, %d, %+v)", puzzleID, userID, r)
					}
					return nil
				}, okStats, func(ctx context.Context, puzzleID string, userID int64) (*model.Rating, error) {
					return &model.Rating{Stars: 5, Difficulty: 4}, nil
				}),
			}},
			Input: &EndpointReq{PuzzleID: "p1", Stars: 5, Difficulty: 4},
			WantResp: &EndpointResp{
				RatingSummary: puzzles.NewRatingSummary(stats),
				Mine:          &model.MyRating{Stars: 5, Difficulty: 4},
			},
		},
	})
}
