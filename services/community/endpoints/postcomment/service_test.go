package postcomment

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/community/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

func okAuth(t *testing.T) *mp.UserAuth {
	return &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*accounts.User, error) {
		return &accounts.User{ID: 7, Username: "ann"}, nil
	}}
}

func limit(t *testing.T, ok bool) *mp.Limiter {
	return &mp.Limiter{T: t, AllowFunc: func(key string) bool {
		if key != "user:ann" {
			t.Errorf("limit key %q", key)
		}
		return ok
	}}
}

func puzzleOf(t *testing.T, p *puzzles.Puzzle, err error) *mp.Puzzle {
	return &mp.Puzzle{T: t, FindFunc: func(ctx context.Context, id string) (*puzzles.Puzzle, error) { return p, err }}
}

func TestNew(t *testing.T) {
	auth, db, lim := &mp.UserAuth{T: t}, &DB{}, &mp.Limiter{T: t}
	if ep := New(auth, db, lim); ep.auth != auth || ep.db != db || ep.limit != lim {
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
		{Name: "empty", Input: &EndpointReq{}, WantErr: true},
		{Name: "blank", Input: &EndpointReq{Text: "   \n"}, WantErr: true},
		{Name: "too long", Input: &EndpointReq{Text: strings.Repeat("x", model.MaxComment+1)}, WantErr: true},
		{Name: "NUL", Input: &EndpointReq{Text: "a\x00b"}, WantErr: true},
		{Name: "not UTF-8", Input: &EndpointReq{Text: "a\xffb"}, WantErr: true},
		{Name: "the longest", Input: &EndpointReq{Text: strings.Repeat("é", model.MaxComment)}},
		{Name: "plain text", Input: &EndpointReq{Text: "<b>hi</b>"}},
	})

	req := &EndpointReq{Text: "  <b>hi</b>  "}
	if err := req.Validate(); err != nil || req.Text != "<b>hi</b>" {
		t.Fatalf("the text is trimmed: %q %v", req.Text, err)
	}
}

func TestEndpoint_Execute(t *testing.T) {
	published := &puzzles.Puzzle{Status: puzzles.Published}
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	ann := "ann"
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
			Name:       "limit_exceeded",
			Endpoint:   &Endpoint{auth: okAuth(t), db: &DB{}, limit: limit(t, false)},
			Input:      &EndpointReq{},
			WantErr:    true,
			WantStatus: http.StatusTooManyRequests,
			WantCode:   "rate_limited",
		},
		{
			Name:     "db.Puzzle.Find_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{Puzzle: puzzleOf(t, nil, errors.New("dbErr"))}, limit: limit(t, true)},
			Input:    &EndpointReq{},
			WantErr:  true,
		},
		{
			Name:       "db.Puzzle.Find_missing",
			Endpoint:   &Endpoint{auth: okAuth(t), db: &DB{Puzzle: puzzleOf(t, nil, nil)}, limit: limit(t, true)},
			Input:      &EndpointReq{},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name:       "puzzle_hidden",
			Endpoint:   &Endpoint{auth: okAuth(t), db: &DB{Puzzle: puzzleOf(t, &puzzles.Puzzle{Status: puzzles.Published, Moderation: puzzles.ModHidden}, nil)}, limit: limit(t, true)},
			Input:      &EndpointReq{},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name: "db.Comment.Create_error",
			Endpoint: &Endpoint{auth: okAuth(t), limit: limit(t, true), db: &DB{
				Puzzle:  puzzleOf(t, published, nil),
				Comment: &mp.Comment{T: t, CreateFunc: func(ctx context.Context, c *model.Comment) error { return errors.New("dbErr") }},
			}},
			Input:   &EndpointReq{},
			WantErr: true,
		},
		{
			Name: "success",
			Endpoint: &Endpoint{auth: okAuth(t), limit: limit(t, true), db: &DB{
				Puzzle: puzzleOf(t, published, nil),
				Comment: &mp.Comment{T: t, CreateFunc: func(ctx context.Context, c *model.Comment) error {
					if c.PuzzleID != "p1" || c.UserID != 7 || c.Body != "hi" || c.Moderation != puzzles.ModNone {
						t.Errorf("Create(%+v)", c)
					}
					c.ID, c.CreatedAt = 5, now
					return nil
				}},
			}},
			Input: &EndpointReq{PuzzleID: "p1", Text: "hi"},
			WantResp: &EndpointResp{CommentView: model.CommentView{
				ID: 5, Author: &ann, Text: "hi", CreatedAt: now, Mine: true,
			}},
		},
	})

	if code := (&EndpointResp{}).StatusCode(); code != http.StatusCreated {
		t.Fatalf("status %d", code)
	}
}
