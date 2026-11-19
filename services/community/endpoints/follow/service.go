package follow

import (
	"context"
	"errors"
	"net/http"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/community/model"
)

type EndpointReq struct {
	Name string
}

func (this *EndpointReq) BindPath(req *http.Request) {
	this.Name = req.PathValue("name")
}

// Execute is idempotent: following a creator that is already followed changes nothing.
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
	if creator.ID == me.ID {
		return nil, http_outcome.NewErrorWithCode(http.StatusBadRequest, "self_follow", errors.New("you cannot follow yourself"))
	}

	already, err := this.db.Follow.Exists(ctx, me.ID, creator.ID)
	if err != nil {
		return nil, st.StackTrace(this.db.Follow.Exists, err)
	}

	if already {
		return &http_outcome.NoResponse{}, nil
	}

	_, following, err := this.db.Follow.Counts(ctx, me.ID)
	if err != nil {
		return nil, st.StackTrace(this.db.Follow.Counts, err)
	}

	if following >= model.MaxFollowing {
		return nil, http_outcome.NewErrorWithCode(http.StatusConflict, "follow_limit", errors.New("you follow too many creators"))
	}

	if err := this.db.Follow.Create(ctx, me.ID, creator.ID); err != nil {
		return nil, st.StackTrace(this.db.Follow.Create, err)
	}

	return &http_outcome.NoResponse{}, nil
}

// The route has no body.
func (this *EndpointReq) BodyFree() {}
