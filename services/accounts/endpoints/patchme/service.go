package patchme

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/accounts/model"
)

type EndpointReq struct {
	Bio *string `json:"bio"`
}

type EndpointResp struct {
	User model.UserJSON `json:"user"`
}

func (this *EndpointReq) Validate() error {
	if this.Bio != nil && !validBio(strings.TrimSpace(*this.Bio)) {
		return http_outcome.NewErrorWithCode(http.StatusBadRequest, "invalid_bio", errors.New("bio must be plain text of at most 500 characters"))
	}

	return nil
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	user, err := this.auth.GetUser(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUser, err)
	}

	if req.Bio != nil {
		bio := strings.TrimSpace(*req.Bio)
		if err := this.db.User.SetBio(ctx, user.ID, bio); err != nil {
			return nil, st.StackTrace(this.db.User.SetBio, err)
		}

		user.Bio = bio
	}

	return &EndpointResp{User: model.NewUserJSON(user)}, nil
}

func validBio(bio string) bool {
	return utf8.RuneCountInString(bio) <= model.MaxBio && utf8.ValidString(bio) && !strings.ContainsRune(bio, 0)
}
