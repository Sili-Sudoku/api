package unfollow

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
)

func okAuth(t *testing.T) *mp.UserAuth {
	return &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*accounts.User, error) { return &accounts.User{ID: 7}, nil }}
}

func creatorDB(t *testing.T, u *accounts.User, err error) *mp.User {
	return &mp.User{T: t, FindByNameFunc: func(ctx context.Context, name string) (*accounts.User, error) { return u, err }}
}

func TestNew(t *testing.T) {
	auth, db := &mp.UserAuth{T: t}, &DB{}
	if ep := New(auth, db); ep.auth != auth || ep.db != db {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_BindPath(t *testing.T) {
	mp.RunBindPathTests(t, []mp.BindPathTest{
		{Name: "name", PathValues: map[string]string{"name": "maker"}, Input: &EndpointReq{}, Want: &EndpointReq{Name: "maker"}},
	})
}

func TestEndpoint_Execute(t *testing.T) {
	maker := &accounts.User{ID: 3}
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, http_outcome.NoResponse]{
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
			Name:     "db.User.FindByName_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{User: creatorDB(t, nil, errors.New("dbErr"))}},
			Input:    &EndpointReq{Name: "maker"},
			WantErr:  true,
		},
		{
			Name:       "db.User.FindByName_missing",
			Endpoint:   &Endpoint{auth: okAuth(t), db: &DB{User: creatorDB(t, nil, nil)}},
			Input:      &EndpointReq{Name: "maker"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name: "db.Follow.Delete_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{User: creatorDB(t, maker, nil),
				Follow: &mp.Follow{T: t, DeleteFunc: func(ctx context.Context, followerID, creatorID int64) error { return errors.New("dbErr") }}}},
			Input:   &EndpointReq{Name: "maker"},
			WantErr: true,
		},
		{
			Name: "success",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{User: creatorDB(t, maker, nil),
				Follow: &mp.Follow{T: t, DeleteFunc: func(ctx context.Context, followerID, creatorID int64) error {
					if followerID != 7 || creatorID != 3 {
						t.Errorf("Delete(%d, %d)", followerID, creatorID)
					}
					return nil
				}}}},
			Input:    &EndpointReq{Name: "maker"},
			WantResp: &http_outcome.NoResponse{},
		},
	})
}
