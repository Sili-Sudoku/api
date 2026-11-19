package claim

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

	deleteToken string
}

func (this *EndpointReq) BindPath(req *http.Request) {
	this.ID = req.PathValue("id")
}

func (this *EndpointReq) BindRequest(req *http.Request) {
	this.deleteToken = req.Header.Get("X-Delete-Token")
}

// Execute attaches an anonymous puzzle to the signed-in user. The delete token proves that the caller made it.
func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *http_outcome.NoResponse, err error) {
	user, err := this.auth.GetUser(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUser, err)
	}

	p, err := this.db.Puzzle.Find(ctx, req.ID)
	if err != nil {
		return nil, st.StackTrace(this.db.Puzzle.Find, err)
	}

	if p == nil {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	}
	if p.OwnerID == user.ID {
		return &http_outcome.NoResponse{}, nil
	}
	if !p.HasToken(req.deleteToken) {
		return nil, http_outcome.NewError(http.StatusForbidden, errors.New("not the owner, and the delete token is missing or wrong"))
	}
	if p.OwnerID != 0 {
		return nil, http_outcome.NewErrorWithCode(http.StatusConflict, "already_owned", errors.New("this puzzle already has an owner"))
	}

	if err := this.db.Puzzle.SetOwner(ctx, req.ID, user.ID); errors.Is(err, model.ErrNotFound) {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	} else if err != nil {
		return nil, st.StackTrace(this.db.Puzzle.SetOwner, err)
	}

	return &http_outcome.NoResponse{}, nil
}

// The route has no body.
func (this *EndpointReq) BodyFree() {}
