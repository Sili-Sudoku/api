package getprofile

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
)

type EndpointReq struct {
	Name string
}

type EndpointResp struct {
	Username  string    `json:"username"`
	Bio       string    `json:"bio"` // plain text
	CreatedAt time.Time `json:"createdAt"`
	Followers int       `json:"followers"`
	Following int       `json:"following"`
	// Followed is whether the signed-in user follows this creator. Always false when signed out.
	Followed bool `json:"followed"`
	Me       bool `json:"me"`
}

func (this *EndpointReq) BindPath(req *http.Request) {
	this.Name = req.PathValue("name")
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	me := this.auth.FindUser(ctx)

	creator, err := this.db.User.FindByName(ctx, req.Name)
	if err != nil {
		return nil, st.StackTrace(this.db.User.FindByName, err)
	}

	if creator == nil {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	}

	followers, following, err := this.db.Follow.Counts(ctx, creator.ID)
	if err != nil {
		return nil, st.StackTrace(this.db.Follow.Counts, err)
	}

	resp = &EndpointResp{Username: creator.Username, Bio: creator.Bio, CreatedAt: creator.CreatedAt, Followers: followers, Following: following}
	if me != nil {
		resp.Me = me.ID == creator.ID
		resp.Followed, err = this.db.Follow.Exists(ctx, me.ID, creator.ID)
		if err != nil {
			return nil, st.StackTrace(this.db.Follow.Exists, err)
		}
	}

	return resp, nil
}
