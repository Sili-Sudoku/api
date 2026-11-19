package follow

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/community/model"
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
	exists := func(already bool, err error) func(ctx context.Context, followerID, creatorID int64) (bool, error) {
		return func(ctx context.Context, followerID, creatorID int64) (bool, error) { return already, err }
	}
	following := func(n int, err error) func(ctx context.Context, userID int64) (int, int, error) {
		return func(ctx context.Context, userID int64) (int, int, error) { return 0, n, err }
	}
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
			Name:       "yourself",
			Endpoint:   &Endpoint{auth: okAuth(t), db: &DB{User: creatorDB(t, &accounts.User{ID: 7}, nil)}},
			Input:      &EndpointReq{Name: "me"},
			WantErr:    true,
			WantStatus: http.StatusBadRequest,
			WantCode:   "self_follow",
		},
		{
			Name: "db.Follow.Exists_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{User: creatorDB(t, maker, nil),
				Follow: &mp.Follow{T: t, ExistsFunc: exists(false, errors.New("dbErr"))}}},
			Input:   &EndpointReq{Name: "maker"},
			WantErr: true,
		},
		{
			Name: "db.Follow.Counts_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{User: creatorDB(t, maker, nil),
				Follow: &mp.Follow{T: t, ExistsFunc: exists(false, nil), CountsFunc: following(0, errors.New("dbErr"))}}},
			Input:   &EndpointReq{Name: "maker"},
			WantErr: true,
		},
		{
			Name: "follow_limit",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{User: creatorDB(t, maker, nil),
				Follow: &mp.Follow{T: t, ExistsFunc: exists(false, nil), CountsFunc: following(model.MaxFollowing, nil)}}},
			Input:      &EndpointReq{Name: "maker"},
			WantErr:    true,
			WantStatus: http.StatusConflict,
			WantCode:   "follow_limit",
		},
		{
			Name: "db.Follow.Create_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{User: creatorDB(t, maker, nil),
				Follow: &mp.Follow{T: t, ExistsFunc: exists(false, nil), CountsFunc: following(0, nil),
					CreateFunc: func(ctx context.Context, followerID, creatorID int64) error { return errors.New("dbErr") }}}},
			Input:   &EndpointReq{Name: "maker"},
			WantErr: true,
		},
		{
			Name: "success_already_following_changes_nothing",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{User: creatorDB(t, maker, nil),
				Follow: &mp.Follow{T: t, ExistsFunc: exists(true, nil)}}},
			Input:    &EndpointReq{Name: "maker"},
			WantResp: &http_outcome.NoResponse{},
		},
		{
			Name: "success",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{User: creatorDB(t, maker, nil),
				Follow: &mp.Follow{T: t, ExistsFunc: exists(false, nil), CountsFunc: following(model.MaxFollowing-1, nil),
					CreateFunc: func(ctx context.Context, followerID, creatorID int64) error {
						if followerID != 7 || creatorID != 3 {
							t.Errorf("Create(%d, %d)", followerID, creatorID)
						}
						return nil
					}}}},
			Input:    &EndpointReq{Name: "maker"},
			WantResp: &http_outcome.NoResponse{},
		},
	})
}
