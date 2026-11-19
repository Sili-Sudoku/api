package listfollowing

import (
	"context"
	"errors"
	"testing"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/community/model"
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

func TestEndpoint_Execute(t *testing.T) {
	followsOf := func(ids []int64, err error) *mp.Follow {
		return &mp.Follow{T: t, ListFunc: func(ctx context.Context, followerID int64, limit int) ([]int64, error) {
			if followerID != 7 || limit != model.MaxFollowing {
				t.Errorf("List(%d, %d)", followerID, limit)
			}
			return ids, err
		}}
	}
	usersOf := func(names map[int64]string, err error) *mp.User {
		return &mp.User{T: t, UsernamesFunc: func(ctx context.Context, ids []int64) (map[int64]string, error) { return names, err }}
	}
	mp.RunExecuteTests(t, []mp.ExecuteTest[http_outcome.NoRequest, EndpointResp]{
		{
			Name: "auth.GetUser_error",
			Endpoint: &Endpoint{
				auth: &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*accounts.User, error) { return nil, errors.New("authErr") }},
				db:   &DB{},
			},
			Input:   &http_outcome.NoRequest{},
			WantErr: true,
		},
		{
			Name:     "db.Follow.List_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{Follow: followsOf(nil, errors.New("dbErr"))}},
			Input:    &http_outcome.NoRequest{},
			WantErr:  true,
		},
		{
			Name:     "db.User.Usernames_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{Follow: followsOf([]int64{3}, nil), User: usersOf(nil, errors.New("dbErr"))}},
			Input:    &http_outcome.NoRequest{},
			WantErr:  true,
		},
		{
			Name:     "success_keeps_the_order_and_skips_gone_accounts",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{Follow: followsOf([]int64{4, 9, 3}, nil), User: usersOf(map[int64]string{3: "maker", 4: "bob"}, nil)}},
			Input:    &http_outcome.NoRequest{},
			WantResp: &EndpointResp{Items: []Item{{Username: "bob"}, {Username: "maker"}}},
		},
		{
			Name:     "success_empty",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{Follow: followsOf(nil, nil), User: usersOf(nil, nil)}},
			Input:    &http_outcome.NoRequest{},
			WantResp: &EndpointResp{Items: []Item{}},
		},
	})
}
