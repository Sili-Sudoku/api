package deleteme

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	"github.com/Sili-Sudoku/api/libs/password"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/accounts/session"
)

const maxBody = 16 << 10

type EndpointReq struct {
	Password string `json:"password"`
	// Puzzles is what happens to the puzzles of the user: "keep" (they lose their owner) or "delete".
	Puzzles string `json:"puzzles"`
}

// EndpointResp is a 204 that clears the cookie.
type EndpointResp struct {
	cookie *http.Cookie
}

func (this *EndpointResp) SetCookies() []*http.Cookie { return []*http.Cookie{this.cookie} }
func (this *EndpointResp) StatusCode() int            { return http.StatusNoContent }

// BindBody is needed because the adapter does not read the body of a DELETE.
func (this *EndpointReq) BindBody(req *http.Request) error {
	return json.NewDecoder(http.MaxBytesReader(nil, req.Body, maxBody)).Decode(this)
}

func (this *EndpointReq) Validate() error {
	if this.Puzzles != "keep" && this.Puzzles != "delete" {
		return http_outcome.NewErrorWithCode(http.StatusBadRequest, "puzzles_choice", errors.New(`puzzles must be "keep" or "delete"`))
	}

	return nil
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	user, err := this.auth.GetUser(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUser, err)
	}

	if !this.limit.Allow(user.Username) {
		return nil, http_outcome.NewErrorWithCode(http.StatusTooManyRequests, "rate_limited", errors.New("too many attempts; try again later"))
	}
	if !password.Check(user.PasswordHash, req.Password) {
		return nil, http_outcome.NewErrorWithCode(http.StatusUnauthorized, "bad_credentials", errors.New("wrong password"))
	}

	if err := this.db.User.Delete(ctx, user.ID, req.Puzzles == "delete"); err != nil {
		return nil, st.StackTrace(this.db.User.Delete, err)
	}

	return &EndpointResp{cookie: session.ClearCookie(this.cfg.InsecureCookie)}, nil
}
