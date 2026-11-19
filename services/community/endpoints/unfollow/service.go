package unfollow

import (
	"context"
	"errors"
	"net/http"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
)

type EndpointReq struct {
	Name string
}

func (this *EndpointReq) BindPath(req *http.Request) {
	this.Name = req.PathValue("name")
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *http_outcome.NoResponse, err error) {
	me, err := this.auth.GetUser(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUser, err)
	}

	creator, err := this.db.User.FindByName(ctx, req.Name)
	if err != nil {
		return nil, st.StackTrace(this.db.User.FindByName, err)
	}

	if creator == nil {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	}

	if err := this.db.Follow.Delete(ctx, me.ID, creator.ID); err != nil {
		return nil, st.StackTrace(this.db.Follow.Delete, err)
	}

	return &http_outcome.NoResponse{}, nil
}
