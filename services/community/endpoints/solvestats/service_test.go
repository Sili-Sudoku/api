package solvestats

import (
	"context"
	"errors"
	"net/http"
	"testing"

	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

func ptr[T any](v T) *T { return &v }

func puzzleOf(t *testing.T, p *puzzles.Puzzle, err error) *mp.Puzzle {
	return &mp.Puzzle{T: t, FindFunc: func(ctx context.Context, id string) (*puzzles.Puzzle, error) { return p, err }}
}

func timesOf(t *testing.T, secs []int, err error) *mp.SolveTimes {
	return &mp.SolveTimes{T: t, SolveSecondsFunc: func(ctx context.Context, puzzleID string) ([]int, error) { return secs, err }}
}

func TestNew(t *testing.T) {
	db := &DB{}
	if ep := New(db); ep.db != db {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_BindPath(t *testing.T) {
	mp.RunBindPathTests(t, []mp.BindPathTest{
		{Name: "id", PathValues: map[string]string{"id": "p1"}, Input: &EndpointReq{}, Want: &EndpointReq{PuzzleID: "p1"}},
	})
}

func TestEndpoint_Execute(t *testing.T) {
	published := &puzzles.Puzzle{Status: puzzles.Published}
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name:     "db.Puzzle.Find_error",
			Endpoint: &Endpoint{db: &DB{Puzzle: puzzleOf(t, nil, errors.New("dbErr"))}},
			Input:    &EndpointReq{PuzzleID: "p1"},
			WantErr:  true,
		},
		{
			Name:       "db.Puzzle.Find_missing",
			Endpoint:   &Endpoint{db: &DB{Puzzle: puzzleOf(t, nil, nil)}},
			Input:      &EndpointReq{PuzzleID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name:       "puzzle_hidden",
			Endpoint:   &Endpoint{db: &DB{Puzzle: puzzleOf(t, &puzzles.Puzzle{Status: puzzles.Published, Moderation: puzzles.ModHidden}, nil)}},
			Input:      &EndpointReq{PuzzleID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name:     "db.SolveHistory.SolveSeconds_error",
			Endpoint: &Endpoint{db: &DB{Puzzle: puzzleOf(t, published, nil), SolveHistory: timesOf(t, nil, errors.New("dbErr"))}},
			Input:    &EndpointReq{PuzzleID: "p1"},
			WantErr:  true,
		},
		{
			Name:     "success_no_solves",
			Endpoint: &Endpoint{db: &DB{Puzzle: puzzleOf(t, published, nil), SolveHistory: timesOf(t, nil, nil)}},
			Input:    &EndpointReq{PuzzleID: "p1"},
			WantResp: &EndpointResp{},
		},
		{
			Name:     "success_median_of_an_unsorted_list",
			Endpoint: &Endpoint{db: &DB{Puzzle: puzzleOf(t, published, nil), SolveHistory: timesOf(t, []int{300, 100, 200}, nil)}},
			Input:    &EndpointReq{PuzzleID: "p1"},
			WantResp: &EndpointResp{Solves: 3, MedianSeconds: ptr(200)},
		},
		{
			Name:     "success_even_count_takes_the_upper_middle",
			Endpoint: &Endpoint{db: &DB{Puzzle: puzzleOf(t, published, nil), SolveHistory: timesOf(t, []int{300, 100}, nil)}},
			Input:    &EndpointReq{PuzzleID: "p1"},
			WantResp: &EndpointResp{Solves: 2, MedianSeconds: ptr(300)},
		},
	})
}
