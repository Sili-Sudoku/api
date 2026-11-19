package deletecomment

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

// Execute lets the author, or an admin, remove a comment.
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
	if comment.UserID != user.ID && !user.IsAdmin {
		return nil, http_outcome.NewError(http.StatusForbidden, errors.New("not allowed"))
	}

	err = this.db.Comment.Delete(ctx, req.id)
	if errors.Is(err, model.ErrNotFound) {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	}
	if err != nil {
		return nil, st.StackTrace(this.db.Comment.Delete, err)
	}

	return &http_outcome.NoResponse{}, nil
}
