package getprofile

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
)

func authOf(t *testing.T, u *accounts.User) *mp.OptionalAuth {
	return &mp.OptionalAuth{T: t, FindUserFunc: func(ctx context.Context) *accounts.User { return u }}
}

func creatorDB(t *testing.T, u *accounts.User, err error) *mp.User {
	return &mp.User{T: t, FindByNameFunc: func(ctx context.Context, name string) (*accounts.User, error) { return u, err }}
}

func TestNew(t *testing.T) {
	auth, db := &mp.OptionalAuth{T: t}, &DB{}
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
	created := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	maker := &accounts.User{ID: 3, Username: "maker", Bio: "hi", CreatedAt: created}
	counts := func(ctx context.Context, userID int64) (int, int, error) {
		if userID != 3 {
			t.Errorf("Counts(%d)", userID)
		}
		return 5, 2, nil
	}
	follows := &mp.Follow{T: t, CountsFunc: counts}
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name:     "db.User.FindByName_error",
			Endpoint: &Endpoint{auth: authOf(t, nil), db: &DB{User: creatorDB(t, nil, errors.New("dbErr"))}},
			Input:    &EndpointReq{Name: "maker"},
			WantErr:  true,
		},
		{
			Name:       "db.User.FindByName_missing",
			Endpoint:   &Endpoint{auth: authOf(t, nil), db: &DB{User: creatorDB(t, nil, nil)}},
			Input:      &EndpointReq{Name: "maker"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name: "db.Follow.Counts_error",
			Endpoint: &Endpoint{auth: authOf(t, nil), db: &DB{User: creatorDB(t, maker, nil), Follow: &mp.Follow{T: t, CountsFunc: func(ctx context.Context, userID int64) (int, int, error) {
				return 0, 0, errors.New("dbErr")
			}}}},
			Input:   &EndpointReq{Name: "maker"},
			WantErr: true,
		},
		{
			Name: "db.Follow.Exists_error",
			Endpoint: &Endpoint{auth: authOf(t, &accounts.User{ID: 7}), db: &DB{User: creatorDB(t, maker, nil), Follow: &mp.Follow{T: t, CountsFunc: counts,
				ExistsFunc: func(ctx context.Context, followerID, creatorID int64) (bool, error) {
					return false, errors.New("dbErr")
				}}}},
			Input:   &EndpointReq{Name: "maker"},
			WantErr: true,
		},
		{
			Name:     "success_signed_out",
			Endpoint: &Endpoint{auth: authOf(t, nil), db: &DB{User: creatorDB(t, maker, nil), Follow: follows}},
			Input:    &EndpointReq{Name: "maker"},
			WantResp: &EndpointResp{Username: "maker", Bio: "hi", CreatedAt: created, Followers: 5, Following: 2},
		},
		{
			Name: "success_signed_in_follower",
			Endpoint: &Endpoint{auth: authOf(t, &accounts.User{ID: 7}), db: &DB{User: creatorDB(t, maker, nil), Follow: &mp.Follow{T: t, CountsFunc: counts,
				ExistsFunc: func(ctx context.Context, followerID, creatorID int64) (bool, error) {
					return followerID == 7 && creatorID == 3, nil
				}}}},
			Input:    &EndpointReq{Name: "maker"},
			WantResp: &EndpointResp{Username: "maker", Bio: "hi", CreatedAt: created, Followers: 5, Following: 2, Followed: true},
		},
		{
			Name: "success_me",
			Endpoint: &Endpoint{auth: authOf(t, maker), db: &DB{User: creatorDB(t, maker, nil), Follow: &mp.Follow{T: t, CountsFunc: counts,
				ExistsFunc: func(ctx context.Context, followerID, creatorID int64) (bool, error) { return false, nil }}}},
			Input:    &EndpointReq{Name: "maker"},
			WantResp: &EndpointResp{Username: "maker", Bio: "hi", CreatedAt: created, Followers: 5, Following: 2, Me: true},
		},
	})
}
