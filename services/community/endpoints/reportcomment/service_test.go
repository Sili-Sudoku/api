package reportcomment

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

func authOf(t *testing.T, u *accounts.User) *mp.UserAuth {
	return &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*accounts.User, error) { return u, nil }}
}

func TestNew(t *testing.T) {
	auth, db := &mp.UserAuth{T: t}, &DB{}
	if ep := New(auth, db); ep.auth != auth || ep.db != db {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_BindPath(t *testing.T) {
	mp.RunBindPathTests(t, []mp.BindPathTest{
		{Name: "id", PathValues: map[string]string{"id": "12"}, Input: &EndpointReq{}, Want: &EndpointReq{rawID: "12"}},
	})
}

func TestEndpointReq_Validate(t *testing.T) {
	mp.RunValidateTests(t, []mp.ValidateTest{
		{Name: "not a number", Input: &EndpointReq{rawID: "x"}, WantErr: true},
		{Name: "zero", Input: &EndpointReq{rawID: "0"}, WantErr: true},
		{Name: "negative", Input: &EndpointReq{rawID: "-3"}, WantErr: true},
		{Name: "positive", Input: &EndpointReq{rawID: "12"}},
	})

	err := (&EndpointReq{rawID: "x"}).Validate()
	var oe *http_outcome.ErrorResponse
	if !errors.As(err, &oe) || oe.Status != http.StatusNotFound || oe.Code != "not_found" {
		t.Fatalf("a bad id is a 404: %v", err)
	}
}

func TestEndpoint_Execute(t *testing.T) {
	ann := &accounts.User{ID: 7}
	commentOf := func(c *model.Comment, err error, report func(ctx context.Context, id, userID int64) (int, error)) *mp.Comment {
		return &mp.Comment{T: t, FindFunc: func(ctx context.Context, id int64) (*model.Comment, error) { return c, err }, AddReportFunc: report}
	}
	theirs := &model.Comment{ID: 3, UserID: 8}
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, http_outcome.NoResponse]{
		{
			Name: "auth.GetUser_error",
			Endpoint: &Endpoint{
				auth: &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*accounts.User, error) { return nil, errors.New("authErr") }},
				db:   &DB{},
			},
			Input:   &EndpointReq{id: 3},
			WantErr: true,
		},
		{
			Name:     "db.Comment.Find_error",
			Endpoint: &Endpoint{auth: authOf(t, ann), db: &DB{Comment: commentOf(nil, errors.New("dbErr"), nil)}},
			Input:    &EndpointReq{id: 3},
			WantErr:  true,
		},
		{
			Name:       "db.Comment.Find_missing",
			Endpoint:   &Endpoint{auth: authOf(t, ann), db: &DB{Comment: commentOf(nil, nil, nil)}},
			Input:      &EndpointReq{id: 3},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name:       "own_comment",
			Endpoint:   &Endpoint{auth: authOf(t, &accounts.User{ID: 8}), db: &DB{Comment: commentOf(&model.Comment{ID: 3, UserID: 8}, nil, nil)}},
			Input:      &EndpointReq{id: 3},
			WantErr:    true,
			WantStatus: http.StatusForbidden,
			WantCode:   "forbidden",
		},
		{
			Name: "db.Comment.AddReport_error",
			Endpoint: &Endpoint{auth: authOf(t, ann), db: &DB{Comment: commentOf(theirs, nil, func(ctx context.Context, id, userID int64) (int, error) {
				return 0, errors.New("dbErr")
			})}},
			Input:   &EndpointReq{id: 3},
			WantErr: true,
		},
		{
			Name: "db.Comment.AddReport_gone_meanwhile",
			Endpoint: &Endpoint{auth: authOf(t, ann), db: &DB{Comment: commentOf(theirs, nil, func(ctx context.Context, id, userID int64) (int, error) {
				return 0, model.ErrNotFound
			})}},
			Input:      &EndpointReq{id: 3},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
		},
		{
			Name: "success",
			Endpoint: &Endpoint{auth: authOf(t, ann), db: &DB{Comment: commentOf(theirs, nil, func(ctx context.Context, id, userID int64) (int, error) {
				if id != 3 || userID != 7 {
					t.Errorf("AddReport(%d, %d)", id, userID)
				}
				return 1, nil
			})}},
			Input:    &EndpointReq{id: 3},
			WantResp: &http_outcome.NoResponse{},
		},
	})
}
