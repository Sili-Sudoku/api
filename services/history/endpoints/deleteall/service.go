package deleteall

import (
	"context"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
)

func (this *Endpoint) Execute(ctx context.Context, _ *http_outcome.NoRequest) (resp *http_outcome.NoResponse, err error) {
	userID, err := this.auth.GetUserID(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUserID, err)
	}

	if err := this.db.SolveHistory.DeleteAll(ctx, userID); err != nil {
		return nil, st.StackTrace(this.db.SolveHistory.DeleteAll, err)
	}

	return &http_outcome.NoResponse{}, nil
}
