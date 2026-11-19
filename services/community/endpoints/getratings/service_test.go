package getratings

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

func signedIn(t *testing.T) *mp.OptionalAuth {
	return &mp.OptionalAuth{T: t, FindUserFunc: func(ctx context.Context) *accounts.User { return &accounts.User{ID: 7} }}
}

func signedOut(t *testing.T) *mp.OptionalAuth {
	return &mp.OptionalAuth{T: t, FindUserFunc: func(ctx context.Context) *accounts.User { return nil }}
}

func puzzleDB(t *testing.T, p *puzzles.Puzzle, err error) *mp.Puzzle {
	return &mp.Puzzle{T: t, FindFunc: func(ctx context.Context, id string) (*puzzles.Puzzle, error) { return p, err }}
}

func TestNew(t *testing.T) {
	auth, db := &mp.OptionalAuth{T: t}, &DB{}
	if ep := New(auth, db); ep.auth != auth || ep.db != db {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_BindPath(t *testing.T) {
	mp.RunBindPathTests(t, []mp.BindPathTest{
		{Name: "id", PathValues: map[string]string{"id": "p1"}, Input: &EndpointReq{}, Want: &EndpointReq{PuzzleID: "p1"}},
	})
}

func TestEndpoint_Execute(t *testing.T) {
	published := &puzzles.Puzzle{Status: puzzles.Published, Ratings: puzzles.RatingStats{StarsCount: 2, StarsSum: 9}}
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name:     "db.Puzzle.Find_error",
			Endpoint: &Endpoint{auth: signedOut(t), db: &DB{Puzzle: puzzleDB(t, nil, errors.New("dbErr"))}},
			Input:    &EndpointReq{PuzzleID: "p1"},
			WantErr:  true,
		},
		{
			Name:       "db.Puzzle.Find_missing",
			Endpoint:   &Endpoint{auth: signedOut(t), db: &DB{Puzzle: puzzleDB(t, nil, nil)}},
			Input:      &EndpointReq{PuzzleID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name:       "puzzle_hidden",
			Endpoint:   &Endpoint{auth: signedOut(t), db: &DB{Puzzle: puzzleDB(t, &puzzles.Puzzle{Status: puzzles.Published, Moderation: puzzles.ModHidden}, nil)}},
			Input:      &EndpointReq{PuzzleID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name: "db.Rating.Find_error",
			Endpoint: &Endpoint{auth: signedIn(t), db: &DB{
				Puzzle: puzzleDB(t, published, nil),
				Rating: &mp.Rating{T: t, FindFunc: func(ctx context.Context, puzzleID string, userID int64) (*model.Rating, error) {
					return nil, errors.New("dbErr")
				}},
			}},
			Input:   &EndpointReq{PuzzleID: "p1"},
			WantErr: true,
		},
		{
			Name:     "success_signed_out_has_no_mine",
			Endpoint: &Endpoint{auth: signedOut(t), db: &DB{Puzzle: puzzleDB(t, published, nil)}},
			Input:    &EndpointReq{PuzzleID: "p1"},
			WantResp: &EndpointResp{RatingSummary: puzzles.NewRatingSummary(published.Ratings)},
		},
		{
			Name: "success_signed_in",
			Endpoint: &Endpoint{auth: signedIn(t), db: &DB{
				Puzzle: puzzleDB(t, published, nil),
				Rating: &mp.Rating{T: t, FindFunc: func(ctx context.Context, puzzleID string, userID int64) (*model.Rating, error) {
					if puzzleID != "p1" || userID != 7 {
						t.Errorf("Find(%q, %d)", puzzleID, userID)
					}
					return &model.Rating{Stars: 4, Difficulty: 2}, nil
				}},
			}},
			Input: &EndpointReq{PuzzleID: "p1"},
			WantResp: &EndpointResp{
				RatingSummary: puzzles.NewRatingSummary(published.Ratings),
				Mine:          &model.MyRating{Stars: 4, Difficulty: 2},
			},
		},
	})
}
