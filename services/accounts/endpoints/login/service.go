package login

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Sili-Sudoku/api/libs/clientip"
	"github.com/Sili-Sudoku/api/libs/http_outcome"
	"github.com/Sili-Sudoku/api/libs/password"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/accounts/session"
)

type EndpointReq struct {
	Username string `json:"username"`
	Password string `json:"password"`

	remoteAddr   string
	forwardedFor string
}

type EndpointResp struct {
	User model.UserJSON `json:"user"`

	cookie *http.Cookie
}

func (this *EndpointResp) SetCookies() []*http.Cookie { return []*http.Cookie{this.cookie} }

func (this *EndpointReq) BindRequest(req *http.Request) {
	this.remoteAddr, this.forwardedFor = req.RemoteAddr, req.Header.Get("X-Forwarded-For")
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	ip := clientip.Resolve(req.remoteAddr, req.forwardedFor, this.cfg.TrustProxy)
	if !this.limitIP.Allow(ip) || !this.limitUser.Allow(req.Username) {
		return nil, http_outcome.NewErrorWithCode(http.StatusTooManyRequests, "rate_limited", errors.New("too many attempts; try again later"))
	}

	u, err := this.db.User.FindByName(ctx, req.Username)
	if err != nil {
		return nil, st.StackTrace(this.db.User.FindByName, err)
	}

	if u == nil {
		_ = password.Check(this.dummy, req.Password)
		return nil, badCredentials()
	}
	if !password.Check(u.PasswordHash, req.Password) {
		return nil, badCredentials()
	}

	token, tokenHash, err := session.New()
	if err != nil {
		return nil, st.StackTrace(session.New, err)
	}

	expires := time.Now().Add(session.TTL)
	if err := this.db.Session.Create(ctx, tokenHash, u.ID, expires); err != nil {
		return nil, st.StackTrace(this.db.Session.Create, err)
	}

	return &EndpointResp{User: model.NewUserJSON(u), cookie: session.Cookie(token, expires, this.cfg.InsecureCookie)}, nil
}

func badCredentials() error {
	return http_outcome.NewErrorWithCode(http.StatusUnauthorized, "bad_credentials", errors.New("wrong username or password"))
}
