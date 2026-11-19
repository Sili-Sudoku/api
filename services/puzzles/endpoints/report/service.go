package report

import (
	"context"
	"errors"
	"net/http"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

type EndpointReq struct {
	ID string
}

func (this *EndpointReq) BindPath(req *http.Request) {
	this.ID = req.PathValue("id")
}

// Execute records a report by a signed-in user. A second report by the same user does not count.
func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *http_outcome.NoResponse, err error) {
	user, err := this.auth.GetUser(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUser, err)
	}

	if _, err := this.db.Puzzle.AddReport(ctx, req.ID, user.ID); errors.Is(err, model.ErrNotFound) {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	} else if err != nil {
		return nil, st.StackTrace(this.db.Puzzle.AddReport, err)
	}

	return &http_outcome.NoResponse{}, nil
}

// The route has no body.
func (this *EndpointReq) BodyFree() {}
