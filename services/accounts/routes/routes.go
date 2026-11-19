package routes

import (
	"net/http"
	"time"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	"github.com/Sili-Sudoku/api/libs/ratelimit"
	"github.com/Sili-Sudoku/api/libs/server"
	"github.com/Sili-Sudoku/api/services/accounts/endpoints/deleteme"
	"github.com/Sili-Sudoku/api/services/accounts/endpoints/getme"
	"github.com/Sili-Sudoku/api/services/accounts/endpoints/login"
	"github.com/Sili-Sudoku/api/services/accounts/endpoints/logout"
	"github.com/Sili-Sudoku/api/services/accounts/endpoints/patchme"
	"github.com/Sili-Sudoku/api/services/accounts/endpoints/signup"
	accountspg "github.com/Sili-Sudoku/api/services/accounts/postgres"
)

// Limits are the rate limits of this program. NewLimits gives the production values.
type Limits struct {
	SignUp    *ratelimit.Limiter
	LogInIP   *ratelimit.Limiter
	LogInUser *ratelimit.Limiter
}

func NewLimits() Limits {
	return Limits{
		SignUp:    ratelimit.New(10, time.Hour),
		LogInIP:   ratelimit.New(30, 10*time.Minute),
		LogInUser: ratelimit.New(10, 10*time.Minute),
	}
}

type Deps struct {
	Config   server.Config
	Limits   Limits
	Sessions *accountspg.Session
	Users    *accountspg.User
}

// New serves sign-up, sessions and /v1/me.
func New(d Deps) http.Handler {
	return server.New(d.Sessions, func(mux *http.ServeMux) { mount(d, mux) })
}

func mount(d Deps, mux *http.ServeMux) {
	auth, insecure := server.Auth{}, d.Config.InsecureCookie

	mux.HandleFunc("POST /v1/accounts", http_outcome.AdaptEndpoint(signup.New(
		&signup.DB{Session: d.Sessions, User: d.Users}, d.Limits.SignUp,
		signup.Config{InsecureCookie: insecure, TrustProxy: d.Config.TrustProxy}).Execute))
	mux.HandleFunc("POST /v1/sessions", http_outcome.AdaptEndpoint(login.New(
		&login.DB{Session: d.Sessions, User: d.Users}, d.Limits.LogInIP, d.Limits.LogInUser,
		login.Config{InsecureCookie: insecure, TrustProxy: d.Config.TrustProxy}).Execute))
	mux.HandleFunc("DELETE /v1/sessions/current", http_outcome.AdaptEndpoint(logout.New(
		&logout.DB{Session: d.Sessions}, logout.Config{InsecureCookie: insecure}).Execute))
	mux.HandleFunc("GET /v1/me", http_outcome.AdaptAuthenticatedEndpoint(auth, getme.New(auth).Execute))
	mux.HandleFunc("PATCH /v1/me", http_outcome.AdaptAuthenticatedEndpoint(auth, patchme.New(auth, &patchme.DB{User: d.Users}).Execute))
	mux.HandleFunc("DELETE /v1/me", http_outcome.AdaptAuthenticatedEndpoint(auth, deleteme.New(
		auth, &deleteme.DB{User: d.Users}, d.Limits.LogInUser, deleteme.Config{InsecureCookie: insecure}).Execute))
}
