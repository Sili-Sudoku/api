package listcomments

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Sili-Sudoku/api/libs/cursor"
	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/community/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

func signedIn(t *testing.T) *mp.OptionalAuth {
	return &mp.OptionalAuth{T: t, FindUserFunc: func(ctx context.Context) *accounts.User { return &accounts.User{ID: 7} }}
}

func puzzleOf(t *testing.T, p *puzzles.Puzzle, err error) *mp.Puzzle {
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

func TestEndpointReq_Validate(t *testing.T) {
	mp.RunValidateTests(t, []mp.ValidateTest{
		{Name: "limit is not a number", Input: &EndpointReq{rawLimit: "x"}, WantErr: true},
		{Name: "limit is zero", Input: &EndpointReq{rawLimit: "0"}, WantErr: true},
		{Name: "limit is too big", Input: &EndpointReq{rawLimit: "51"}, WantErr: true},
		{Name: "cursor is not a cursor", Input: &EndpointReq{rawCursor: "zzz"}, WantErr: true},
		{Name: "cursor without an id", Input: &EndpointReq{rawCursor: cursor.Encode(&model.CommentCursor{})}, WantErr: true},
		{Name: "defaults", Input: &EndpointReq{}},
		{Name: "limit and cursor", Input: &EndpointReq{rawLimit: "50", rawCursor: cursor.Encode(&model.CommentCursor{ID: 3})}},
	})
}

func TestEndpoint_Execute(t *testing.T) {
	t0 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	published := &puzzles.Puzzle{Status: puzzles.Published}
	comments := []*model.Comment{{ID: 3, UserID: 7, Body: "mine", CreatedAt: t0.Add(time.Hour)}, {ID: 2, UserID: 8, Body: "gone", CreatedAt: t0}}
	commentsOf := func(cs []*model.Comment, err error) *mp.Comment {
		return &mp.Comment{T: t, ListFunc: func(ctx context.Context, puzzleID string, after *model.CommentCursor, limit int) ([]*model.Comment, error) {
			return cs, err
		}}
	}
	usersOf := func(names map[int64]string, err error) *mp.User {
		return &mp.User{T: t, UsernamesFunc: func(ctx context.Context, ids []int64) (map[int64]string, error) { return names, err }}
	}
	ann := "ann"

	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name:     "db.Puzzle.Find_error",
			Endpoint: &Endpoint{auth: signedIn(t), db: &DB{Puzzle: puzzleOf(t, nil, errors.New("dbErr"))}},
			Input:    &EndpointReq{limit: 20},
			WantErr:  true,
		},
		{
			Name:       "db.Puzzle.Find_missing",
			Endpoint:   &Endpoint{auth: signedIn(t), db: &DB{Puzzle: puzzleOf(t, nil, nil)}},
			Input:      &EndpointReq{limit: 20},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name:       "puzzle_hidden",
			Endpoint:   &Endpoint{auth: signedIn(t), db: &DB{Puzzle: puzzleOf(t, &puzzles.Puzzle{Status: puzzles.Published, Moderation: puzzles.ModHidden}, nil)}},
			Input:      &EndpointReq{limit: 20},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name:     "db.Comment.List_error",
			Endpoint: &Endpoint{auth: signedIn(t), db: &DB{Puzzle: puzzleOf(t, published, nil), Comment: commentsOf(nil, errors.New("dbErr"))}},
			Input:    &EndpointReq{limit: 20},
			WantErr:  true,
		},
		{
			Name: "db.User.Usernames_error",
			Endpoint: &Endpoint{auth: signedIn(t), db: &DB{
				Puzzle: puzzleOf(t, published, nil), Comment: commentsOf(comments, nil), User: usersOf(nil, errors.New("dbErr")),
			}},
			Input:   &EndpointReq{limit: 20},
			WantErr: true,
		},
		{
			Name: "success_signed_in_marks_mine_and_a_gone_author_is_null",
			Endpoint: &Endpoint{auth: signedIn(t), db: &DB{
				Puzzle: puzzleOf(t, published, nil), Comment: commentsOf(comments, nil), User: usersOf(map[int64]string{7: "ann"}, nil),
			}},
			Input: &EndpointReq{limit: 20},
			WantResp: &EndpointResp{Items: []model.CommentView{
				{ID: 3, Author: &ann, Text: "mine", CreatedAt: t0.Add(time.Hour), Mine: true},
				{ID: 2, Text: "gone", CreatedAt: t0},
			}},
		},
		{
			Name: "success_signed_out_marks_nothing_mine",
			Endpoint: &Endpoint{
				auth: &mp.OptionalAuth{T: t, FindUserFunc: func(ctx context.Context) *accounts.User { return nil }},
				db: &DB{
					Puzzle: puzzleOf(t, published, nil), Comment: commentsOf(comments[:1], nil), User: usersOf(map[int64]string{7: "ann"}, nil),
				},
			},
			Input: &EndpointReq{limit: 20},
			WantResp: &EndpointResp{Items: []model.CommentView{
				{ID: 3, Author: &ann, Text: "mine", CreatedAt: t0.Add(time.Hour)},
			}},
		},
	})

	t.Run("success_next_cursor", func(t *testing.T) {
		ep := &Endpoint{auth: signedIn(t), db: &DB{
			Puzzle: puzzleOf(t, published, nil), Comment: commentsOf(comments, nil), User: usersOf(nil, nil),
		}}
		resp, err := ep.Execute(context.Background(), &EndpointReq{limit: 1})
		if err != nil || len(resp.Items) != 1 || resp.Items[0].ID != 3 {
			t.Fatalf("page: %+v %v", resp, err)
		}
		next, ok := cursor.Decode[model.CommentCursor](resp.NextCursor)
		if !ok || next.ID != 3 || !next.CreatedAt.Equal(t0.Add(time.Hour)) {
			t.Fatalf("next cursor: %+v", next)
		}
	})
}
