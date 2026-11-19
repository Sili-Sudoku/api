package signup

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
	db, limit, cfg := &DB{}, &mp.Limiter{T: t}, Config{TrustProxy: true}
	if ep := New(db, limit, cfg); ep.db != db || ep.limit != limit || ep.cfg != cfg {
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

func TestEndpointReq_Validate(t *testing.T) {
	long := strings.Repeat("a", model.MaxPassword+1)
	const pw = "long enough pw"
	mp.RunValidateTests(t, []mp.ValidateTest{
		{Name: "username too short", Input: &EndpointReq{Username: "ab", Password: pw}, WantErr: true},
		{Name: "username with a space", Input: &EndpointReq{Username: "has space", Password: pw}, WantErr: true},
		{Name: "username is markup", Input: &EndpointReq{Username: "<script>", Password: pw}, WantErr: true},
		{Name: "username too long", Input: &EndpointReq{Username: strings.Repeat("a", 25), Password: pw}, WantErr: true},
		{Name: "password too short", Input: &EndpointReq{Username: "okname", Password: "short"}, WantErr: true},
		{Name: "password too long", Input: &EndpointReq{Username: "okname", Password: long}, WantErr: true},
		{Name: "email without at", Input: &EndpointReq{Username: "okname", Password: pw, Email: "no-at-sign"}, WantErr: true},
		{Name: "email with two ats", Input: &EndpointReq{Username: "okname", Password: pw, Email: "a@b@c.de"}, WantErr: true},
		{Name: "no email", Input: &EndpointReq{Username: "okname", Password: pw}},
		{Name: "email is trimmed", Input: &EndpointReq{Username: "okname", Password: pw, Email: " ann@example.com "}},
	})
}

func TestEndpoint_Execute(t *testing.T) {
	okUser := func(t *testing.T) *mp.User {
		return &mp.User{T: t, CreateFunc: func(ctx context.Context, u *model.User) error { u.ID = 7; return nil }}
	}
	req := func() *EndpointReq {
		return &EndpointReq{Username: "ann_1", Password: "correct horse", Email: " ann@example.com "}
	}
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name:       "limit.Allow_refused",
			Endpoint:   &Endpoint{limit: &mp.Limiter{T: t, AllowFunc: func(key string) bool { return false }}, db: &DB{}},
			Input:      req(),
			WantErr:    true,
			WantStatus: http.StatusTooManyRequests,
			WantCode:   "rate_limited",
		},
		{
			Name: "db.User.Create_taken",
			Endpoint: &Endpoint{limit: allow(t), db: &DB{User: &mp.User{T: t, CreateFunc: func(ctx context.Context, u *model.User) error {
				return model.ErrUsernameTaken
			}}}},
			Input:      req(),
			WantErr:    true,
			WantStatus: http.StatusConflict,
			WantCode:   "username_taken",
		},
		{
			Name: "db.User.Create_error",
			Endpoint: &Endpoint{limit: allow(t), db: &DB{User: &mp.User{T: t, CreateFunc: func(ctx context.Context, u *model.User) error {
				return errors.New("dbErr")
			}}}},
			Input:   req(),
			WantErr: true,
		},
		{
			Name: "db.Session.Create_error",
			Endpoint: &Endpoint{limit: allow(t), db: &DB{User: okUser(t), Session: &mp.Session{T: t, CreateFunc: func(ctx context.Context, hash [32]byte, userID int64, expires time.Time) error {
				return errors.New("dbErr")
			}}}},
			Input:   req(),
			WantErr: true,
		},
	})
}

func TestEndpoint_Execute_success(t *testing.T) {
	var gotUser *model.User
	var gotHash [32]byte
	ep := &Endpoint{
		cfg: Config{InsecureCookie: true},
		limit: &mp.Limiter{T: t, AllowFunc: func(key string) bool {
			if key != "1.2.3.4" {
				t.Errorf("limit key = %q", key)
			}
			return true
		}},
		db: &DB{
			User: &mp.User{T: t, CreateFunc: func(ctx context.Context, u *model.User) error { u.ID = 7; gotUser = u; return nil }},
			Session: &mp.Session{T: t, CreateFunc: func(ctx context.Context, hash [32]byte, userID int64, expires time.Time) error {
				if userID != 7 || time.Until(expires) < session.TTL-time.Minute {
					t.Errorf("session for %d until %v", userID, expires)
				}
				gotHash = hash
				return nil
			}},
		},
	}
	resp, err := ep.Execute(context.Background(), &EndpointReq{Username: "ann_1", Password: "correct horse", Email: " ann@example.com ", remoteAddr: "1.2.3.4:5"})
	if err != nil {
		t.Fatal(err)
	}
	if gotUser.Email != "ann@example.com" || !password.Check(gotUser.PasswordHash, "correct horse") {
		t.Fatalf("stored user: %+v", gotUser)
	}
	if resp.User.Username != "ann_1" || resp.User.Email != "ann@example.com" || resp.StatusCode() != http.StatusCreated {
		t.Fatalf("resp: %+v", resp)
	}
	cookies := resp.SetCookies()
	if len(cookies) != 1 || cookies[0].Name != session.CookieName || session.Hash(cookies[0].Value) != gotHash || cookies[0].Secure {
		t.Fatalf("cookie: %+v", cookies)
	}
}
