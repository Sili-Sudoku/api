package list

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sili-Sudoku/api/libs/cursor"
	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	"github.com/Sili-Sudoku/api/services/history/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

func okAuth(t *testing.T) *mp.Auth {
	return &mp.Auth{T: t, GetUserIDFunc: func(ctx context.Context) (int64, error) { return 7, nil }}
}

func TestNew(t *testing.T) {
	auth, db := &mp.Auth{T: t}, &DB{}
	if ep := New(auth, db); ep.auth != auth || ep.db != db {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_Validate(t *testing.T) {
	mp.RunValidateTests(t, []mp.ValidateTest{
		{Name: "limit is not a number", Input: &EndpointReq{rawLimit: "x"}, WantErr: true},
		{Name: "limit is zero", Input: &EndpointReq{rawLimit: "0"}, WantErr: true},
		{Name: "limit is too big", Input: &EndpointReq{rawLimit: "51"}, WantErr: true},
		{Name: "cursor is not a cursor", Input: &EndpointReq{rawCursor: "zzz"}, WantErr: true},
		{Name: "cursor without an id", Input: &EndpointReq{rawCursor: cursor.Encode(&model.Cursor{})}, WantErr: true},
		{Name: "defaults", Input: &EndpointReq{}},
		{Name: "limit and cursor", Input: &EndpointReq{rawLimit: "50", rawCursor: cursor.Encode(&model.Cursor{PuzzleID: "p1"})}},
	})
}

func TestEndpoint_Execute(t *testing.T) {
	t0 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	entries := []model.Entry{{PuzzleID: "p2", UpdatedAt: t0.Add(time.Hour)}, {PuzzleID: "p1", UpdatedAt: t0}}
	title := "Nice one"
	puzzleDB := &mp.Puzzle{T: t, FindFunc: func(ctx context.Context, id string) (*puzzles.Puzzle, error) {
		if id == "p1" {
			return &puzzles.Puzzle{Title: title, Status: puzzles.Published, Moderation: puzzles.ModNone}, nil
		}
		return nil, nil
	}}
	listOf := func(es []model.Entry, err error) *mp.SolveHistory {
		return &mp.SolveHistory{T: t, ListFunc: func(ctx context.Context, userID int64, after *model.Cursor, limit int) ([]model.Entry, error) {
			return es, err
		}}
	}

	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name: "auth.GetUserID_error",
			Endpoint: &Endpoint{
				auth: &mp.Auth{T: t, GetUserIDFunc: func(ctx context.Context) (int64, error) { return 0, errors.New("authErr") }},
				db:   &DB{},
			},
			Input:   &EndpointReq{limit: 20},
			WantErr: true,
		},
		{
			Name:     "db.SolveHistory.List_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{SolveHistory: listOf(nil, errors.New("dbErr"))}},
			Input:    &EndpointReq{limit: 20},
			WantErr:  true,
		},
		{
			Name:     "success_last_page_with_a_title_only_for_a_visible_puzzle",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{Puzzle: puzzleDB, SolveHistory: listOf(entries, nil)}},
			Input:    &EndpointReq{limit: 2},
			WantResp: &EndpointResp{Items: []Item{
				{PuzzleID: "p2", UpdatedAt: t0.Add(time.Hour)},
				{PuzzleID: "p1", Title: &title, UpdatedAt: t0},
			}},
		},
		{
			Name:     "success_empty",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{SolveHistory: listOf(nil, nil)}},
			Input:    &EndpointReq{limit: 20},
			WantResp: &EndpointResp{Items: []Item{}},
		},
	})

	t.Run("success_next_cursor", func(t *testing.T) {
		ep := &Endpoint{auth: okAuth(t), db: &DB{Puzzle: puzzleDB, SolveHistory: listOf(entries, nil)}}
		resp, err := ep.Execute(context.Background(), &EndpointReq{limit: 1})
		if err != nil || len(resp.Items) != 1 || resp.Items[0].PuzzleID != "p2" {
			t.Fatalf("page: %+v %v", resp, err)
		}
		next, ok := cursor.Decode[model.Cursor](resp.NextCursor)
		if !ok || next.PuzzleID != "p2" || !next.UpdatedAt.Equal(t0.Add(time.Hour)) {
			t.Fatalf("next cursor: %+v", next)
		}
	})
}
