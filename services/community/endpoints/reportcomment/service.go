package reportcomment

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/community/model"
)

type EndpointReq struct {
	rawID string

	id int64
}

func (this *EndpointReq) BindPath(req *http.Request) {
	this.rawID = req.PathValue("id")
}

func (this *EndpointReq) Validate() error {
	id, err := strconv.ParseInt(this.rawID, 10, 64)
	if err != nil || id <= 0 {
		return http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	}
	this.id = id

	return nil
}

// Execute records a report. A user cannot report their own comment, and a second report by the same user does not count.
func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *http_outcome.NoResponse, err error) {
	user, err := this.auth.GetUser(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUser, err)
	}

	comment, err := this.db.Comment.Find(ctx, req.id)
	if err != nil {
		return nil, st.StackTrace(this.db.Comment.Find, err)
	}

	if comment == nil {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	}
	if comment.UserID == user.ID {
		return nil, http_outcome.NewError(http.StatusForbidden, errors.New("not allowed"))
	}

	_, err = this.db.Comment.AddReport(ctx, req.id, user.ID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	}
	if err != nil {
		return nil, st.StackTrace(this.db.Comment.AddReport, err)
	}

	return &http_outcome.NoResponse{}, nil
}

// The route has no body.
func (this *EndpointReq) BodyFree() {}
