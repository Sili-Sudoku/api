package getme

import (
	"context"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/accounts/model"
)

type EndpointResp struct {
	User model.UserJSON `json:"user"`
}

func (this *Endpoint) Execute(ctx context.Context, _ *http_outcome.NoRequest) (resp *EndpointResp, err error) {
	user, err := this.auth.GetUser(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUser, err)
	}

	return &EndpointResp{User: model.NewUserJSON(user)}, nil
}
