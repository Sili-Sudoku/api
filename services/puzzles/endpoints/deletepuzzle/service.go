package deletepuzzle

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

// Execute deletes for the owner, or for the holder of the delete token.
func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *http_outcome.NoResponse, err error) {
	p, err := this.db.Puzzle.Find(ctx, req.ID)
	if err != nil {
		return nil, st.StackTrace(this.db.Puzzle.Find, err)
	}

	if p == nil {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	}

	user := this.auth.FindUser(ctx)
	if !(user != nil && p.OwnerID == user.ID) && !p.HasToken(req.deleteToken) {
		return nil, http_outcome.NewError(http.StatusForbidden, errors.New("not the owner, and the delete token is missing or wrong"))
	}

	if err := this.db.Puzzle.Delete(ctx, req.ID); errors.Is(err, model.ErrNotFound) {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	} else if err != nil {
		return nil, st.StackTrace(this.db.Puzzle.Delete, err)
	}

	return &http_outcome.NoResponse{}, nil
}
