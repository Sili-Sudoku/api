package listfollowing

import (
	"context"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/community/model"
)

type EndpointResp struct {
	Items []Item `json:"items"`
}

type Item struct {
	Username string `json:"username"`
}

// Execute lists the creators the user follows, newest follow first. A creator whose account is gone is left out.
func (this *Endpoint) Execute(ctx context.Context, _ *http_outcome.NoRequest) (resp *EndpointResp, err error) {
	me, err := this.auth.GetUser(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUser, err)
	}

	ids, err := this.db.Follow.List(ctx, me.ID, model.MaxFollowing)
	if err != nil {
		return nil, st.StackTrace(this.db.Follow.List, err)
	}

	names, err := this.db.User.Usernames(ctx, ids)
	if err != nil {
		return nil, st.StackTrace(this.db.User.Usernames, err)
	}

	resp = &EndpointResp{Items: make([]Item, 0, len(ids))}
	for _, id := range ids {
		if name, ok := names[id]; ok {
			resp.Items = append(resp.Items, Item{Username: name})
		}
	}

	return resp, nil
}
