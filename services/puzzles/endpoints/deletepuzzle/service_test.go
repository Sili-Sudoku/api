package deletepuzzle

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

// user is a signed-in user with this id, or nobody for 0.
func user(id int64) *mp.OptionalAuth {
	return &mp.OptionalAuth{FindUserFunc: func(ctx context.Context) *accounts.User {
		if id == 0 {
			return nil
		}
		return &accounts.User{ID: id}
	}}
}

func puzzle(owner int64) *model.Puzzle {
	return &model.Puzzle{ID: "p1", OwnerID: owner, TokenHash: sha256.Sum256([]byte("secret"))}
}

func TestNew(t *testing.T) {
	auth, db := &mp.OptionalAuth{T: t}, &DB{}
	if ep := New(auth, db); ep.auth != auth || ep.db != db {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_BindPath(t *testing.T) {
	mp.RunBindPathTests(t, []mp.BindPathTest{
		{Name: "id", PathValues: map[string]string{"id": "p1"}, Input: &EndpointReq{}, Want: &EndpointReq{ID: "p1"}},
	})
}

func TestEndpointReq_BindRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r.Header.Set("X-Delete-Token", "secret")
	var req EndpointReq
	req.BindRequest(r)
	if req.deleteToken != "secret" {
		t.Fatalf("bound %+v", req)
	}
}

func TestEndpoint_Execute(t *testing.T) {
	db := func(p *model.Puzzle, findErr, delErr error) *DB {
		return &DB{Puzzle: &mp.Puzzle{
			T:          t,
			FindFunc:   func(ctx context.Context, id string) (*model.Puzzle, error) { return p, findErr },
			DeleteFunc: func(ctx context.Context, id string) error { return delErr },
		}}
	}
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, http_outcome.NoResponse]{
		{
			Name:     "db.Puzzle.Find_error",
			Endpoint: &Endpoint{auth: user(0), db: db(nil, errors.New("dbErr"), nil)},
			Input:    &EndpointReq{ID: "p1"},
			WantErr:  true,
		},
		{
			Name:       "db.Puzzle.Find_missing",
			Endpoint:   &Endpoint{auth: user(0), db: db(nil, nil, nil)},
			Input:      &EndpointReq{ID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
			WantCode:   "not_found",
		},
		{
			Name:       "signed_out_without_a_token",
			Endpoint:   &Endpoint{auth: user(0), db: db(puzzle(0), nil, nil)},
			Input:      &EndpointReq{ID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusForbidden,
			WantCode:   "forbidden",
		},
		{
			Name:       "wrong_token",
			Endpoint:   &Endpoint{auth: user(0), db: db(puzzle(0), nil, nil)},
			Input:      &EndpointReq{ID: "p1", deleteToken: "wrong"},
			WantErr:    true,
			WantStatus: http.StatusForbidden,
		},
		{
			Name:       "another_user_without_a_token",
			Endpoint:   &Endpoint{auth: user(8), db: db(puzzle(7), nil, nil)},
			Input:      &EndpointReq{ID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusForbidden,
		},
		{
			Name:       "a_user_is_not_the_owner_of_an_anonymous_puzzle",
			Endpoint:   &Endpoint{auth: user(8), db: db(puzzle(0), nil, nil)},
			Input:      &EndpointReq{ID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusForbidden,
		},
		{
			Name:       "db.Puzzle.Delete_missing",
			Endpoint:   &Endpoint{auth: user(7), db: db(puzzle(7), nil, model.ErrNotFound)},
			Input:      &EndpointReq{ID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name:     "db.Puzzle.Delete_error",
			Endpoint: &Endpoint{auth: user(7), db: db(puzzle(7), nil, errors.New("dbErr"))},
			Input:    &EndpointReq{ID: "p1"},
			WantErr:  true,
		},
		{
			Name:     "success_owner",
			Endpoint: &Endpoint{auth: user(7), db: db(puzzle(7), nil, nil)},
			Input:    &EndpointReq{ID: "p1"},
			WantResp: &http_outcome.NoResponse{},
		},
		{
			Name:     "success_token_signed_out",
			Endpoint: &Endpoint{auth: user(0), db: db(puzzle(0), nil, nil)},
			Input:    &EndpointReq{ID: "p1", deleteToken: "secret"},
			WantResp: &http_outcome.NoResponse{},
		},
	})
}
