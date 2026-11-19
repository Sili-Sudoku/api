package report

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
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
		{Name: "id", PathValues: map[string]string{"id": "p1"}, Input: &EndpointReq{}, Want: &EndpointReq{ID: "p1"}},
	})
}

func TestEndpoint_Execute(t *testing.T) {
	reports := func(n int, err error) *DB {
		return &DB{Puzzle: &mp.Puzzle{T: t, AddReportFunc: func(ctx context.Context, id string, userID int64) (int, error) {
			if id != "p1" || userID != 7 {
				t.Errorf("AddReport(%q, %d)", id, userID)
			}
			return n, err
		}}}
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
			Name:       "db.Puzzle.AddReport_missing",
			Endpoint:   &Endpoint{auth: okAuth(t), db: reports(0, model.ErrNotFound)},
			Input:      &EndpointReq{ID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
			WantCode:   "not_found",
		},
		{
			Name:     "db.Puzzle.AddReport_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: reports(0, errors.New("dbErr"))},
			Input:    &EndpointReq{ID: "p1"},
			WantErr:  true,
		},
		{
			Name:     "success",
			Endpoint: &Endpoint{auth: okAuth(t), db: reports(1, nil)},
			Input:    &EndpointReq{ID: "p1"},
			WantResp: &http_outcome.NoResponse{},
		},
	})
}
