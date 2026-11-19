package login

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	"github.com/Sili-Sudoku/api/libs/password"
	"github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/accounts/session"
)

func allow(t *testing.T) *mp.Limiter {
	return &mp.Limiter{T: t, AllowFunc: func(key string) bool { return true }}
}

func TestNew(t *testing.T) {
	db, ip, user, cfg := &DB{}, &mp.Limiter{T: t}, &mp.Limiter{T: t}, Config{TrustProxy: true}
	ep := New(db, ip, user, cfg)
	if ep.db != db || ep.limitIP != ip || ep.limitUser != user || ep.cfg != cfg || !password.Check(ep.dummy, "dummy-password-for-timing") {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_BindRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "1.2.3.4:5"
	r.Header.Set("X-Forwarded-For", "9.9.9.9")
	req := &EndpointReq{}
	req.BindRequest(r)
	if req.remoteAddr != "1.2.3.4:5" || req.forwardedFor != "9.9.9.9" {
		t.Fatalf("bound %+v", req)
	}
}

func TestEndpoint_Execute(t *testing.T) {
	hash, _ := password.Hash("correct horse")
	dummy, _ := password.Hash("dummy-password-for-timing")
	ann := &model.User{ID: 7, Username: "ann_1", PasswordHash: hash}
	find := func(u *model.User, err error) *mp.User {
		return &mp.User{T: t, FindByNameFunc: func(ctx context.Context, name string) (*model.User, error) { return u, err }}
	}
	req := func(pw string) *EndpointReq { return &EndpointReq{Username: "ann_1", Password: pw} }
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name: "limitIP.Allow_refused",
			Endpoint: &Endpoint{
				limitIP: &mp.Limiter{T: t, AllowFunc: func(key string) bool { return false }},
				db:      &DB{},
			},
			Input:      req("correct horse"),
			WantErr:    true,
			WantStatus: http.StatusTooManyRequests,
			WantCode:   "rate_limited",
		},
		{
			Name: "limitUser.Allow_refused",
			Endpoint: &Endpoint{
				limitIP:   allow(t),
				limitUser: &mp.Limiter{T: t, AllowFunc: func(key string) bool { return false }},
				db:        &DB{},
			},
			Input:      req("correct horse"),
			WantErr:    true,
			WantStatus: http.StatusTooManyRequests,
			WantCode:   "rate_limited",
		},
		{
			Name:     "db.User.FindByName_error",
			Endpoint: &Endpoint{limitIP: allow(t), limitUser: allow(t), db: &DB{User: find(nil, errors.New("dbErr"))}},
			Input:    req("correct horse"),
			WantErr:  true,
		},
		{
			Name:       "db.User.FindByName_unknown_user",
			Endpoint:   &Endpoint{limitIP: allow(t), limitUser: allow(t), dummy: dummy, db: &DB{User: find(nil, nil)}},
			Input:      req("correct horse"),
			WantErr:    true,
			WantStatus: http.StatusUnauthorized,
			WantCode:   "bad_credentials",
		},
		{
			Name:       "wrong_password",
			Endpoint:   &Endpoint{limitIP: allow(t), limitUser: allow(t), dummy: dummy, db: &DB{User: find(ann, nil)}},
			Input:      req("wrong password"),
			WantErr:    true,
			WantStatus: http.StatusUnauthorized,
			WantCode:   "bad_credentials",
		},
		{
			Name: "db.Session.Create_error",
			Endpoint: &Endpoint{limitIP: allow(t), limitUser: allow(t), dummy: dummy, db: &DB{User: find(ann, nil), Session: &mp.Session{T: t,
				CreateFunc: func(ctx context.Context, hash [32]byte, userID int64, expires time.Time) error {
					return errors.New("dbErr")
				}}}},
			Input:   req("correct horse"),
			WantErr: true,
		},
	})
}

func TestEndpoint_Execute_success(t *testing.T) {
	hash, _ := password.Hash("correct horse")
	var gotHash [32]byte
	ep := &Endpoint{
		cfg: Config{TrustProxy: true},
		limitIP: &mp.Limiter{T: t, AllowFunc: func(key string) bool {
			if key != "9.9.9.9" {
				t.Errorf("ip key = %q", key)
			}
			return true
		}},
		limitUser: &mp.Limiter{T: t, AllowFunc: func(key string) bool {
			if key != "ann_1" {
				t.Errorf("user key = %q", key)
			}
			return true
		}},
		db: &DB{
			User: &mp.User{T: t, FindByNameFunc: func(ctx context.Context, name string) (*model.User, error) {
				return &model.User{ID: 7, Username: "ann_1", PasswordHash: hash}, nil
			}},
			Session: &mp.Session{T: t, CreateFunc: func(ctx context.Context, h [32]byte, userID int64, expires time.Time) error {
				if userID != 7 {
					t.Errorf("session for %d", userID)
				}
				gotHash = h
				return nil
			}},
		},
	}
	resp, err := ep.Execute(context.Background(), &EndpointReq{Username: "ann_1", Password: "correct horse", remoteAddr: "1.2.3.4:5", forwardedFor: "8.8.8.8, 9.9.9.9"})
	if err != nil {
		t.Fatal(err)
	}
	cookies := resp.SetCookies()
	if resp.User.Username != "ann_1" || len(cookies) != 1 || session.Hash(cookies[0].Value) != gotHash || !cookies[0].Secure {
		t.Fatalf("resp: %+v cookies %+v", resp, cookies)
	}
}
