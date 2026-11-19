// Package server is what every area's routes share: the CSRF check, loading the signed-in user from the session cookie,
// the {error, code} body and the mux with its health check. Routes themselves live with each area (services/<area>/routes).
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/accounts/session"
)

const (
	csrfHeader = "X-Requested-With"
	csrfValue  = "sudoku"
)

type Config struct {
	// InsecureCookie drops the Secure flag from the session cookie. Browsers send a Secure cookie to
	// http://localhost, so this is only for a dev setup on another host name.
	InsecureCookie bool
	// TrustProxy reads the client address from X-Forwarded-For. Turn it on only behind a proxy that sets it.
	TrustProxy bool
}

// ConfigFromEnv reads COOKIE_INSECURE and TRUST_PROXY.
func ConfigFromEnv() Config {
	return Config{
		InsecureCookie: os.Getenv("COOKIE_INSECURE") == "true",
		TrustProxy:     os.Getenv("TRUST_PROXY") == "true",
	}
}

// Sessions finds the user of a session token hash. Every program needs it, because a session made by the accounts
// program is accepted by all of them.
type Sessions interface {
	FindUser(ctx context.Context, hash [32]byte, now time.Time) (*accounts.User, error)
}

type ctxKey struct{}

// New builds the handler of one program: a mux with GET /healthz and whatever mount adds, behind the middleware.
func New(sessions Sessions, mount func(mux *http.ServeMux)) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mount(mux)

	return middleware(sessions, mux)
}

// middleware adds the CSRF check and loads the signed-in user, if any, into the request context.
func middleware(sessions Sessions, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			// A cross-site form cannot set a custom header, and a cross-site fetch needs a CORS preflight
			// that this API never grants.
			if r.Header.Get(csrfHeader) != csrfValue {
				writeError(w, http.StatusForbidden, "csrf", "missing "+csrfHeader+" header")
				return
			}
		}

		if c, err := r.Cookie(session.CookieName); err == nil && c.Value != "" {
			u, err := sessions.FindUser(r.Context(), session.Hash(c.Value), time.Now())
			if err != nil {
				log.Printf("authenticate: %v", err)
			} else if u != nil {
				r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, u))
			}
		}

		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg, "code": code})
}

// Auth reads the user that the middleware put in the request context. Every endpoint declares the few methods it needs.
type Auth struct{}

func (Auth) FindUser(ctx context.Context) *accounts.User {
	u, _ := ctx.Value(ctxKey{}).(*accounts.User)
	return u
}

func (a Auth) GetUser(ctx context.Context) (*accounts.User, error) {
	u := a.FindUser(ctx)
	if u == nil {
		return nil, http_outcome.NewErrorWithCode(http.StatusUnauthorized, "unauthenticated", errors.New("sign in first"))
	}

	return u, nil
}

func (a Auth) GetUserID(ctx context.Context) (int64, error) {
	u, err := a.GetUser(ctx)
	if err != nil {
		return 0, err
	}

	return u.ID, nil
}

// Authenticate is the check that runs before anything else is read from the request.
func (a Auth) Authenticate(ctx context.Context) error {
	_, err := a.GetUser(ctx)
	return err
}
