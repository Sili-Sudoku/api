package routes

import (
	"net/http"
	"time"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	"github.com/Sili-Sudoku/api/libs/ratelimit"
	"github.com/Sili-Sudoku/api/libs/server"
	accountspg "github.com/Sili-Sudoku/api/services/accounts/postgres"
	"github.com/Sili-Sudoku/api/services/puzzles/endpoints/claim"
	"github.com/Sili-Sudoku/api/services/puzzles/endpoints/deletepuzzle"
	"github.com/Sili-Sudoku/api/services/puzzles/endpoints/getpuzzle"
	"github.com/Sili-Sudoku/api/services/puzzles/endpoints/list"
	"github.com/Sili-Sudoku/api/services/puzzles/endpoints/publish"
	"github.com/Sili-Sudoku/api/services/puzzles/endpoints/report"
	puzzlespg "github.com/Sili-Sudoku/api/services/puzzles/postgres"
	"github.com/Sili-Sudoku/api/services/puzzles/worker"
)

// Limits are the rate limits of this program. NewLimits gives the production values.
type Limits struct {
	Publish *ratelimit.Limiter
}

func NewLimits() Limits {
	return Limits{
		Publish: ratelimit.New(30, time.Hour),
	}
}

type Deps struct {
	Config   server.Config
	Limits   Limits
	Puzzles  *puzzlespg.Puzzle
	Sessions *accountspg.Session
	Users    *accountspg.User
	Worker   *worker.Worker
}

// New serves publish, browse, get, claim, report and delete.
func New(d Deps) http.Handler {
	return server.New(d.Sessions, func(mux *http.ServeMux) { mount(d, mux) })
}

func mount(d Deps, mux *http.ServeMux) {
	auth := server.Auth{}

	mux.HandleFunc("POST /v1/puzzles", http_outcome.AdaptEndpoint(publish.New(
		auth, &publish.DB{Puzzle: d.Puzzles}, d.Limits.Publish, d.Worker, d.Config.TrustProxy).Execute))
	mux.HandleFunc("GET /v1/puzzles", http_outcome.AdaptEndpoint(list.New(
		&list.DB{Puzzle: d.Puzzles, User: d.Users}).Execute))
	mux.HandleFunc("PUT /v1/puzzles/{id}/owner", http_outcome.AdaptAuthenticatedEndpoint(auth, claim.New(
		auth, &claim.DB{Puzzle: d.Puzzles}).Execute))
	mux.HandleFunc("GET /v1/puzzles/{id}", http_outcome.AdaptEndpoint(getpuzzle.New(
		auth, &getpuzzle.DB{Puzzle: d.Puzzles, User: d.Users}, d.Worker).Execute))
	mux.HandleFunc("POST /v1/puzzles/{id}/report", http_outcome.AdaptAuthenticatedEndpoint(auth, report.New(
		auth, &report.DB{Puzzle: d.Puzzles}).Execute))
	mux.HandleFunc("DELETE /v1/puzzles/{id}", http_outcome.AdaptEndpoint(deletepuzzle.New(
		auth, &deletepuzzle.DB{Puzzle: d.Puzzles}).Execute))
}
