package deleteall

import (
	"context"
	"errors"
	"testing"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
)

func TestNew(t *testing.T) {
	auth, db := &mp.Auth{T: t}, &DB{}
	if ep := New(auth, db); ep.auth != auth || ep.db != db {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpoint_Execute(t *testing.T) {
	okAuth := &mp.Auth{T: t, GetUserIDFunc: func(ctx context.Context) (int64, error) { return 7, nil }}
	mp.RunExecuteTests(t, []mp.ExecuteTest[http_outcome.NoRequest, http_outcome.NoResponse]{
		{
			Name: "auth.GetUserID_error",
			Endpoint: &Endpoint{
				auth: &mp.Auth{T: t, GetUserIDFunc: func(ctx context.Context) (int64, error) { return 0, errors.New("authErr") }},
				db:   &DB{},
			},
			Input:   &http_outcome.NoRequest{},
			WantErr: true,
		},
		{
			Name: "db.SolveHistory.DeleteAll_error",
			Endpoint: &Endpoint{
				auth: okAuth,
				db: &DB{SolveHistory: &mp.SolveHistory{T: t, DeleteAllFunc: func(ctx context.Context, userID int64) error {
					return errors.New("dbErr")
				}}},
			},
			Input:   &http_outcome.NoRequest{},
			WantErr: true,
		},
		{
			Name: "success",
			Endpoint: &Endpoint{
				auth: okAuth,
				db: &DB{SolveHistory: &mp.SolveHistory{T: t, DeleteAllFunc: func(ctx context.Context, userID int64) error {
					if userID != 7 {
						t.Errorf("DeleteAll(%d)", userID)
					}
					return nil
				}}},
			},
			Input:    &http_outcome.NoRequest{},
			WantResp: &http_outcome.NoResponse{},
		},
	})
}
