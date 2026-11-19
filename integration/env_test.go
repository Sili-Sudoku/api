package integration

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sili-Sudoku/api/libs/dbtest"
	"github.com/Sili-Sudoku/api/libs/validate"
	accountspg "github.com/Sili-Sudoku/api/services/accounts/postgres"
	accountsroutes "github.com/Sili-Sudoku/api/services/accounts/routes"
	communitypg "github.com/Sili-Sudoku/api/services/community/postgres"
	communityroutes "github.com/Sili-Sudoku/api/services/community/routes"
	historypg "github.com/Sili-Sudoku/api/services/history/postgres"
	historyroutes "github.com/Sili-Sudoku/api/services/history/routes"
	puzzlespg "github.com/Sili-Sudoku/api/services/puzzles/postgres"
	puzzlesroutes "github.com/Sili-Sudoku/api/services/puzzles/routes"
	"github.com/Sili-Sudoku/api/services/puzzles/worker"
)

// env is the four programs on one database. h serves a request with the program that has the route, as the gateway does.
// A test that needs other limits changes the area's Deps and calls build again.
type env struct {
	h         http.Handler
	worker    *worker.Worker
	accounts  accountsroutes.Deps
	community communityroutes.Deps
	history   historyroutes.Deps
	puzzles   puzzlesroutes.Deps
}

// The tests need TEST_DATABASE_URL and are skipped without it: the stores are Postgres only.
func newEnv(t *testing.T) *env { return newEnvWith(t, validate.Accept{}) }

func newEnvWith(t *testing.T, v validate.Validator) *env {
	t.Helper()

	pool := dbtest.Pool(t)
	puzzles := puzzlespg.NewPuzzle(pool)
	sessions := accountspg.NewSession(pool)
	users := accountspg.NewUser(pool)
	solveHistory := historypg.NewSolveHistory(pool)

	e := &env{worker: worker.New(puzzles, v)}
	e.accounts = accountsroutes.Deps{Limits: accountsroutes.NewLimits(), Sessions: sessions, Users: users}
	e.puzzles = puzzlesroutes.Deps{Limits: puzzlesroutes.NewLimits(), Puzzles: puzzles, Sessions: sessions, Users: users, Worker: e.worker}
	e.community = communityroutes.Deps{
		Comments:     communitypg.NewComment(pool),
		Follows:      communitypg.NewFollow(pool),
		Limits:       communityroutes.NewLimits(),
		Puzzles:      puzzles,
		Ratings:      communitypg.NewRating(pool),
		Sessions:     sessions,
		SolveHistory: solveHistory,
		Users:        users,
	}
	e.history = historyroutes.Deps{Puzzles: puzzles, Sessions: sessions, SolveHistory: solveHistory}
	e.build()

	return e
}

func (e *env) build() {
	e.h = all(accountsroutes.New(e.accounts), puzzlesroutes.New(e.puzzles), communityroutes.New(e.community), historyroutes.New(e.history))
}

// all gives a request to the first program that has a route for it. The programs' own muxes decide, so this needs no
// routing table: programs_test.go checks that exactly one program answers each route.
func all(programs ...http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		for _, program := range programs {
			req := r.Clone(r.Context())
			req.Body = io.NopCloser(bytes.NewReader(body))

			rec := httptest.NewRecorder()
			program.ServeHTTP(rec, req)

			if notRouted(rec.Code, rec.Body.String()) {
				continue
			}

			for k, v := range rec.Header() {
				w.Header()[k] = v
			}

			w.WriteHeader(rec.Code)
			_, _ = w.Write(rec.Body.Bytes())

			return
		}

		http.NotFound(w, r)
	})
}
