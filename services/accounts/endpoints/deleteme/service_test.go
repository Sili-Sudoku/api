package deleteme

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	"github.com/Sili-Sudoku/api/libs/password"
	"github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/accounts/session"
)

func TestNew(t *testing.T) {
	auth, db, limit, cfg := &mp.UserAuth{T: t}, &DB{}, &mp.Limiter{T: t}, Config{InsecureCookie: true}
	if ep := New(auth, db, limit, cfg); ep.auth != auth || ep.db != db || ep.limit != limit || ep.cfg != cfg {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_BindBody(t *testing.T) {
	req := &EndpointReq{}
	r := httptest.NewRequest(http.MethodDelete, "/", strings.NewReader(`{"password":"pw","puzzles":"keep"}`))
	if err := req.BindBody(r); err != nil || *req != (EndpointReq{Password: "pw", Puzzles: "keep"}) {
		t.Fatalf("bound %+v %v", req, err)
	}
	if err := (&EndpointReq{}).BindBody(httptest.NewRequest(http.MethodDelete, "/", strings.NewReader(`nope`))); err == nil {
		t.Fatal("a body that is not JSON must fail")
	}
}

func TestEndpointReq_Validate(t *testing.T) {
	mp.RunValidateTests(t, []mp.ValidateTest{
		{Name: "no choice", Input: &EndpointReq{Password: "pw"}, WantErr: true},
		{Name: "other choice", Input: &EndpointReq{Password: "pw", Puzzles: "maybe"}, WantErr: true},
		{Name: "keep", Input: &EndpointReq{Puzzles: "keep"}},
		{Name: "delete", Input: &EndpointReq{Puzzles: "delete"}},
	})
}

func TestEndpoint_Execute(t *testing.T) {
	hash, _ := password.Hash("correct horse")
	okAuth := &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*model.User, error) {
		return &model.User{ID: 7, Username: "ann", PasswordHash: hash}, nil
	}}
	allow := &mp.Limiter{T: t, AllowFunc: func(key string) bool {
		if key != "ann" {
			t.Errorf("limit key = %q", key)
		}
		return true
	}}
	deleteWith := func(want bool, err error) *DB {
		return &DB{User: &mp.User{T: t, DeleteFunc: func(ctx context.Context, id int64, deletePuzzles bool) error {
			if id != 7 || deletePuzzles != want {
				t.Errorf("Delete(%d, %v)", id, deletePuzzles)
			}
			return err
		}}}
	}
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name: "auth.GetUser_error",
			Endpoint: &Endpoint{
				auth: &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*model.User, error) { return nil, errors.New("authErr") }},
				db:   &DB{},
			},
			Input:   &EndpointReq{Password: "correct horse", Puzzles: "keep"},
			WantErr: true,
		},
		{
			Name: "limit.Allow_refused",
			Endpoint: &Endpoint{
				auth:  okAuth,
				limit: &mp.Limiter{T: t, AllowFunc: func(key string) bool { return false }},
				db:    &DB{},
			},
			Input:      &EndpointReq{Password: "correct horse", Puzzles: "keep"},
			WantErr:    true,
			WantStatus: http.StatusTooManyRequests,
			WantCode:   "rate_limited",
		},
		{
			Name:       "wrong_password",
			Endpoint:   &Endpoint{auth: okAuth, limit: allow, db: &DB{}},
			Input:      &EndpointReq{Password: "wrong password", Puzzles: "keep"},
			WantErr:    true,
			WantStatus: http.StatusUnauthorized,
			WantCode:   "bad_credentials",
		},
		{
			Name:     "db.User.Delete_error",
			Endpoint: &Endpoint{auth: okAuth, limit: allow, db: deleteWith(false, errors.New("dbErr"))},
			Input:    &EndpointReq{Password: "correct horse", Puzzles: "keep"},
			WantErr:  true,
		},
		{
			Name:     "success_keep",
			Endpoint: &Endpoint{auth: okAuth, limit: allow, db: deleteWith(false, nil)},
			Input:    &EndpointReq{Password: "correct horse", Puzzles: "keep"},
			WantResp: &EndpointResp{cookie: session.ClearCookie(false)},
		},
		{
			Name:     "success_delete_puzzles",
			Endpoint: &Endpoint{auth: okAuth, cfg: Config{InsecureCookie: true}, limit: allow, db: deleteWith(true, nil)},
			Input:    &EndpointReq{Password: "correct horse", Puzzles: "delete"},
			WantResp: &EndpointResp{cookie: session.ClearCookie(true)},
		},
	})
}
