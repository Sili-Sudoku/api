package signup

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

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
	Email    string `json:"email"`

	remoteAddr   string
	forwardedFor string
}

type EndpointResp struct {
	User model.UserJSON `json:"user"`

	cookie *http.Cookie
}

func (this *EndpointResp) SetCookies() []*http.Cookie { return []*http.Cookie{this.cookie} }
func (this *EndpointResp) StatusCode() int            { return http.StatusCreated }

func (this *EndpointReq) BindRequest(req *http.Request) {
	this.remoteAddr, this.forwardedFor = req.RemoteAddr, req.Header.Get("X-Forwarded-For")
}

func (this *EndpointReq) Validate() error {
	if !model.UsernamePattern.MatchString(this.Username) {
		return invalid("invalid_username", "username must be 3-24 letters, digits or underscores")
	}
	if n := utf8.RuneCountInString(this.Password); n < model.MinPassword || n > model.MaxPassword {
		return invalid("weak_password", "password must be 10-200 characters")
	}
	if email := strings.TrimSpace(this.Email); email != "" && !validEmail(email) {
		return invalid("invalid_email", "email is not valid")
	}

	return nil
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	if !this.limit.Allow(clientip.Resolve(req.remoteAddr, req.forwardedFor, this.cfg.TrustProxy)) {
		return nil, http_outcome.NewErrorWithCode(http.StatusTooManyRequests, "rate_limited", errors.New("too many attempts; try again later"))
	}

	hash, err := password.Hash(req.Password)
	if err != nil {
		return nil, st.StackTrace(password.Hash, err)
	}

	now := time.Now()
	u := &model.User{Username: req.Username, Email: strings.TrimSpace(req.Email), PasswordHash: hash, CreatedAt: now}
	err = this.db.User.Create(ctx, u)
	if errors.Is(err, model.ErrUsernameTaken) {
		return nil, http_outcome.NewErrorWithCode(http.StatusConflict, "username_taken", err)
	}
	if err != nil {
		return nil, st.StackTrace(this.db.User.Create, err)
	}

	token, tokenHash, err := session.New()
	if err != nil {
		return nil, st.StackTrace(session.New, err)
	}

	expires := now.Add(session.TTL)
	if err := this.db.Session.Create(ctx, tokenHash, u.ID, expires); err != nil {
		return nil, st.StackTrace(this.db.Session.Create, err)
	}

	return &EndpointResp{User: model.NewUserJSON(u), cookie: session.Cookie(token, expires, this.cfg.InsecureCookie)}, nil
}

func invalid(code, msg string) error {
	return http_outcome.NewErrorWithCode(http.StatusBadRequest, code, errors.New(msg))
}

func validEmail(e string) bool {
	if len(e) > 254 || strings.ContainsAny(e, " \t\r\n<>\"") {
		return false
	}
	at := strings.IndexByte(e, '@')

	return at > 0 && at == strings.LastIndexByte(e, '@') && strings.Contains(e[at:], ".") && !strings.HasSuffix(e, ".")
}
