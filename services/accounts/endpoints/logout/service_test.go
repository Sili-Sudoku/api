package logout

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	"github.com/Sili-Sudoku/api/services/accounts/session"
)

func TestNew(t *testing.T) {
	db, cfg := &DB{}, Config{InsecureCookie: true}
	if ep := New(db, cfg); ep.db != db || ep.cfg != cfg {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_BindRequest(t *testing.T) {
	signedIn := httptest.NewRequest(http.MethodDelete, "/", nil)
	signedIn.AddCookie(&http.Cookie{Name: session.CookieName, Value: "tok"})
	for _, tt := range []struct {
		name string
		r    *http.Request
		want string
	}{
		{"cookie", signedIn, "tok"},
		{"no cookie", httptest.NewRequest(http.MethodDelete, "/", nil), ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := &EndpointReq{}
			req.BindRequest(tt.r)
			if req.token != tt.want {
				t.Fatalf("token = %q, want %q", req.token, tt.want)
			}
		})
	}
}

func TestEndpoint_Execute(t *testing.T) {
	cleared := &EndpointResp{cookie: session.ClearCookie(false)}
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name: "db.Session.Delete_error",
			Endpoint: &Endpoint{db: &DB{Session: &mp.Session{T: t, DeleteFunc: func(ctx context.Context, hash [32]byte) error {
				return errors.New("dbErr")
			}}}},
			Input:   &EndpointReq{token: "tok"},
			WantErr: true,
		},
		{
			Name: "success",
			Endpoint: &Endpoint{db: &DB{Session: &mp.Session{T: t, DeleteFunc: func(ctx context.Context, hash [32]byte) error {
				if hash != session.Hash("tok") {
					t.Errorf("Delete(%x)", hash)
				}
				return nil
			}}}},
			Input:    &EndpointReq{token: "tok"},
			WantResp: cleared,
		},
		{
			Name:     "success_signed_out",
			Endpoint: &Endpoint{db: &DB{Session: &mp.Session{T: t}}},
			Input:    &EndpointReq{},
			WantResp: cleared,
		},
	})
}
