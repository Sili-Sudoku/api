package claim

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

func okAuth(t *testing.T) *mp.UserAuth {
	return &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*accounts.User, error) { return &accounts.User{ID: 7}, nil }}
}

func puzzle(owner int64) *model.Puzzle {
	return &model.Puzzle{ID: "p1", OwnerID: owner, TokenHash: sha256.Sum256([]byte("secret"))}
}

func TestNew(t *testing.T) {
	auth, db := &mp.UserAuth{T: t}, &DB{}
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
	r := httptest.NewRequest(http.MethodPut, "/", nil)
	r.Header.Set("X-Delete-Token", "secret")
	var req EndpointReq
	req.BindRequest(r)
	if req.deleteToken != "secret" {
		t.Fatalf("bound %+v", req)
	}
}

func TestEndpoint_Execute(t *testing.T) {
	db := func(p *model.Puzzle, findErr, setErr error) *DB {
		return &DB{Puzzle: &mp.Puzzle{
			T:        t,
			FindFunc: func(ctx context.Context, id string) (*model.Puzzle, error) { return p, findErr },
			SetOwnerFunc: func(ctx context.Context, id string, ownerID int64) error {
				if id != "p1" || ownerID != 7 {
					t.Errorf("SetOwner(%q, %d)", id, ownerID)
				}
				return setErr
			},
		}}
	}
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, http_outcome.NoResponse]{
		{
			Name: "auth.GetUser_error",
			Endpoint: &Endpoint{
				auth: &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*accounts.User, error) { return nil, errors.New("authErr") }},
				db:   &DB{},
			},
			Input:   &EndpointReq{ID: "p1"},
			WantErr: true,
		},
		{
			Name:     "db.Puzzle.Find_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: db(nil, errors.New("dbErr"), nil)},
			Input:    &EndpointReq{ID: "p1"},
			WantErr:  true,
		},
		{
			Name:       "db.Puzzle.Find_missing",
			Endpoint:   &Endpoint{auth: okAuth(t), db: db(nil, nil, nil)},
			Input:      &EndpointReq{ID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
			WantCode:   "not_found",
		},
		{
			Name:     "already_mine_changes_nothing",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{Puzzle: &mp.Puzzle{T: t, FindFunc: func(ctx context.Context, id string) (*model.Puzzle, error) { return puzzle(7), nil }}}},
			Input:    &EndpointReq{ID: "p1"},
			WantResp: &http_outcome.NoResponse{},
		},
		{
			Name:       "wrong_token",
			Endpoint:   &Endpoint{auth: okAuth(t), db: db(puzzle(0), nil, nil)},
			Input:      &EndpointReq{ID: "p1", deleteToken: "wrong"},
			WantErr:    true,
			WantStatus: http.StatusForbidden,
			WantCode:   "forbidden",
		},
		{
			Name:       "missing_token",
			Endpoint:   &Endpoint{auth: okAuth(t), db: db(puzzle(0), nil, nil)},
			Input:      &EndpointReq{ID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusForbidden,
			WantCode:   "forbidden",
		},
		{
			Name:       "owned_by_someone_else",
			Endpoint:   &Endpoint{auth: okAuth(t), db: db(puzzle(8), nil, nil)},
			Input:      &EndpointReq{ID: "p1", deleteToken: "secret"},
			WantErr:    true,
			WantStatus: http.StatusConflict,
			WantCode:   "already_owned",
		},
		{
			Name:       "db.Puzzle.SetOwner_missing",
			Endpoint:   &Endpoint{auth: okAuth(t), db: db(puzzle(0), nil, model.ErrNotFound)},
			Input:      &EndpointReq{ID: "p1", deleteToken: "secret"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name:     "db.Puzzle.SetOwner_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: db(puzzle(0), nil, errors.New("dbErr"))},
			Input:    &EndpointReq{ID: "p1", deleteToken: "secret"},
			WantErr:  true,
		},
		{
			Name:     "success",
			Endpoint: &Endpoint{auth: okAuth(t), db: db(puzzle(0), nil, nil)},
			Input:    &EndpointReq{ID: "p1", deleteToken: "secret"},
			WantResp: &http_outcome.NoResponse{},
		},
	})
}
