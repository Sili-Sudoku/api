package routes

import (
	"net/http"
	"time"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	"github.com/Sili-Sudoku/api/libs/ratelimit"
	"github.com/Sili-Sudoku/api/libs/server"
	accountspg "github.com/Sili-Sudoku/api/services/accounts/postgres"
	"github.com/Sili-Sudoku/api/services/community/endpoints/deletecomment"
	"github.com/Sili-Sudoku/api/services/community/endpoints/feed"
	"github.com/Sili-Sudoku/api/services/community/endpoints/follow"
	"github.com/Sili-Sudoku/api/services/community/endpoints/getprofile"
	"github.com/Sili-Sudoku/api/services/community/endpoints/getratings"
	"github.com/Sili-Sudoku/api/services/community/endpoints/listcomments"
	"github.com/Sili-Sudoku/api/services/community/endpoints/listfollowing"
	"github.com/Sili-Sudoku/api/services/community/endpoints/postcomment"
	"github.com/Sili-Sudoku/api/services/community/endpoints/putrating"
	"github.com/Sili-Sudoku/api/services/community/endpoints/reportcomment"
	"github.com/Sili-Sudoku/api/services/community/endpoints/solvestats"
	"github.com/Sili-Sudoku/api/services/community/endpoints/unfollow"
	communitypg "github.com/Sili-Sudoku/api/services/community/postgres"
	historypg "github.com/Sili-Sudoku/api/services/history/postgres"
	puzzlespg "github.com/Sili-Sudoku/api/services/puzzles/postgres"
)

// Limits are the rate limits of this program. NewLimits gives the production values.
type Limits struct {
	Comment *ratelimit.Limiter
}

func NewLimits() Limits {
	return Limits{
		Comment: ratelimit.New(30, time.Hour),
	}
}

type Deps struct {
	Comments     *communitypg.Comment
	Follows      *communitypg.Follow
	Limits       Limits
	Puzzles      *puzzlespg.Puzzle
	Ratings      *communitypg.Rating
	Sessions     *accountspg.Session
	SolveHistory *historypg.SolveHistory
	Users        *accountspg.User
}

// New serves ratings, comments, profiles, follows and the feed.
func New(d Deps) http.Handler {
	return server.New(d.Sessions, func(mux *http.ServeMux) { mount(d, mux) })
}

func mount(d Deps, mux *http.ServeMux) {
	auth := server.Auth{}

	mux.HandleFunc("GET /v1/puzzles/{id}/ratings", http_outcome.AdaptEndpoint(getratings.New(
		auth, &getratings.DB{Puzzle: d.Puzzles, Rating: d.Ratings}).Execute))
	mux.HandleFunc("PUT /v1/puzzles/{id}/ratings", http_outcome.AdaptAuthenticatedEndpoint(auth, putrating.New(
		auth, &putrating.DB{Puzzle: d.Puzzles, Rating: d.Ratings}).Execute))
	mux.HandleFunc("GET /v1/puzzles/{id}/comments", http_outcome.AdaptEndpoint(listcomments.New(
		auth, &listcomments.DB{Comment: d.Comments, Puzzle: d.Puzzles, User: d.Users}).Execute))
	mux.HandleFunc("POST /v1/puzzles/{id}/comments", http_outcome.AdaptAuthenticatedEndpoint(auth, postcomment.New(
		auth, &postcomment.DB{Comment: d.Comments, Puzzle: d.Puzzles}, d.Limits.Comment).Execute))
	mux.HandleFunc("GET /v1/puzzles/{id}/stats", http_outcome.AdaptEndpoint(solvestats.New(
		&solvestats.DB{Puzzle: d.Puzzles, SolveHistory: d.SolveHistory}).Execute))
	mux.HandleFunc("DELETE /v1/comments/{id}", http_outcome.AdaptAuthenticatedEndpoint(auth, deletecomment.New(
		auth, &deletecomment.DB{Comment: d.Comments}).Execute))
	mux.HandleFunc("POST /v1/comments/{id}/report", http_outcome.AdaptAuthenticatedEndpoint(auth, reportcomment.New(
		auth, &reportcomment.DB{Comment: d.Comments}).Execute))

	mux.HandleFunc("GET /v1/me/following", http_outcome.AdaptAuthenticatedEndpoint(auth, listfollowing.New(
		auth, &listfollowing.DB{Follow: d.Follows, User: d.Users}).Execute))
	mux.HandleFunc("GET /v1/me/feed", http_outcome.AdaptAuthenticatedEndpoint(auth, feed.New(
		auth, &feed.DB{Follow: d.Follows, Puzzle: d.Puzzles, User: d.Users}).Execute))
	mux.HandleFunc("GET /v1/users/{name}", http_outcome.AdaptEndpoint(getprofile.New(
		auth, &getprofile.DB{Follow: d.Follows, User: d.Users}).Execute))
	mux.HandleFunc("POST /v1/users/{name}/follow", http_outcome.AdaptAuthenticatedEndpoint(auth, follow.New(
		auth, &follow.DB{Follow: d.Follows, User: d.Users}).Execute))
	mux.HandleFunc("DELETE /v1/users/{name}/follow", http_outcome.AdaptAuthenticatedEndpoint(auth, unfollow.New(
		auth, &unfollow.DB{Follow: d.Follows, User: d.Users}).Execute))
}
