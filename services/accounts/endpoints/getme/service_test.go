package getme

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	"github.com/Sili-Sudoku/api/services/accounts/model"
)

func TestNew(t *testing.T) {
	auth := &mp.UserAuth{T: t}
	if ep := New(auth); ep.auth != auth {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpoint_Execute(t *testing.T) {
	created := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	mp.RunExecuteTests(t, []mp.ExecuteTest[http_outcome.NoRequest, EndpointResp]{
		{
			Name: "auth.GetUser_error",
			Endpoint: &Endpoint{auth: &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*model.User, error) {
				return nil, errors.New("authErr")
			}}},
			Input:   &http_outcome.NoRequest{},
			WantErr: true,
		},
		{
			Name: "success",
			Endpoint: &Endpoint{auth: &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*model.User, error) {
				return &model.User{ID: 7, Username: "ann", Email: "ann@example.com", Bio: "hi", IsAdmin: true, CreatedAt: created, PasswordHash: "secret"}, nil
			}}},
			Input: &http_outcome.NoRequest{},
			WantResp: &EndpointResp{User: model.UserJSON{Username: "ann", Email: "ann@example.com", Bio: "hi", IsAdmin: true,
				CreatedAt: created}},
		},
	})
}
