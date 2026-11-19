package routes

import (
	"net/http"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	"github.com/Sili-Sudoku/api/libs/server"
	accountspg "github.com/Sili-Sudoku/api/services/accounts/postgres"
	"github.com/Sili-Sudoku/api/services/history/endpoints/deleteall"
	"github.com/Sili-Sudoku/api/services/history/endpoints/list"
	"github.com/Sili-Sudoku/api/services/history/endpoints/record"
	historypg "github.com/Sili-Sudoku/api/services/history/postgres"
	puzzlespg "github.com/Sili-Sudoku/api/services/puzzles/postgres"
)

type Deps struct {
	Puzzles      *puzzlespg.Puzzle
	Sessions     *accountspg.Session
	SolveHistory *historypg.SolveHistory
}

// New serves the solve-history routes.
func New(d Deps) http.Handler {
	return server.New(d.Sessions, func(mux *http.ServeMux) { mount(d, mux) })
}

func mount(d Deps, mux *http.ServeMux) {
	auth := server.Auth{}

	mux.HandleFunc("GET /v1/me/history", http_outcome.AdaptAuthenticatedEndpoint(auth, list.New(
		auth, &list.DB{Puzzle: d.Puzzles, SolveHistory: d.SolveHistory}).Execute))
	mux.HandleFunc("PUT /v1/me/history/{puzzleId}", http_outcome.AdaptAuthenticatedEndpoint(auth, record.New(
		auth, &record.DB{SolveHistory: d.SolveHistory}).Execute))
	mux.HandleFunc("DELETE /v1/me/history", http_outcome.AdaptAuthenticatedEndpoint(auth, deleteall.New(
		auth, &deleteall.DB{SolveHistory: d.SolveHistory}).Execute))
}
