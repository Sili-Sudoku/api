package getratings

import (
	"context"
	"errors"
	"net/http"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/community/model"
)

type EndpointReq struct {
	PuzzleID string
}

type EndpointResp = model.RatingView

func (this *EndpointReq) BindPath(req *http.Request) {
	this.PuzzleID = req.PathValue("id")
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	user := this.auth.FindUser(ctx)

	puzzle, err := this.db.Puzzle.Find(ctx, req.PuzzleID)
	if err != nil {
		return nil, st.StackTrace(this.db.Puzzle.Find, err)
	}

	if puzzle == nil || !puzzle.Visible() {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	}

	var mine *model.Rating
	if user != nil {
		mine, err = this.db.Rating.Find(ctx, req.PuzzleID, user.ID)
		if err != nil {
			return nil, st.StackTrace(this.db.Rating.Find, err)
		}
	}

	view := model.NewRatingView(puzzle.Ratings, mine)

	return &view, nil
}
