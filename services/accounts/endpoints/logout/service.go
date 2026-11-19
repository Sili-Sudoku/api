package logout

import (
	"context"
	"net/http"

	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/accounts/session"
)

type EndpointReq struct {
	token string
}

// EndpointResp is a 204 that clears the cookie.
type EndpointResp struct {
	cookie *http.Cookie
}

func (this *EndpointResp) SetCookies() []*http.Cookie { return []*http.Cookie{this.cookie} }
func (this *EndpointResp) StatusCode() int            { return http.StatusNoContent }

func (this *EndpointReq) BindRequest(req *http.Request) {
	if c, err := req.Cookie(session.CookieName); err == nil {
		this.token = c.Value
	}
}

// Execute works signed out too: without a cookie there is nothing to delete, and the cookie is cleared anyway.
func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	if req.token != "" {
		if err := this.db.Session.Delete(ctx, session.Hash(req.token)); err != nil {
			return nil, st.StackTrace(this.db.Session.Delete, err)
		}
	}

	return &EndpointResp{cookie: session.ClearCookie(this.cfg.InsecureCookie)}, nil
}
