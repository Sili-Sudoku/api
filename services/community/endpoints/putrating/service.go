package putrating

import (
	"context"
	"errors"
	"net/http"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/community/model"
)

type EndpointReq struct {
	Stars      int `json:"stars"`
	Difficulty int `json:"difficulty"`

	PuzzleID string `json:"-"`
}

type EndpointResp = model.RatingView

func (this *EndpointReq) BindPath(req *http.Request) {
	this.PuzzleID = req.PathValue("id")
}

// Each field is 0 (not given) or 1 to 5. A rating with both fields at 0 removes the rating.
func (this *EndpointReq) Validate() error {
	if this.Stars < 0 || this.Stars > 5 || this.Difficulty < 0 || this.Difficulty > 5 {
		return errors.New("stars and difficulty are 1 to 5; a comment has 1 to 2000 characters")
	}

	return nil
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	user, err := this.auth.GetUser(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUser, err)
	}

	puzzle, err := this.db.Puzzle.Find(ctx, req.PuzzleID)
	if err != nil {
		return nil, st.StackTrace(this.db.Puzzle.Find, err)
	}

	if puzzle == nil || !puzzle.Visible() {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	}
	if puzzle.OwnerID != 0 && puzzle.OwnerID == user.ID {
		return nil, http_outcome.NewErrorWithCode(http.StatusForbidden, "own_puzzle", errors.New("you cannot rate your own puzzle"))
	}

	rating := model.Rating{Stars: req.Stars, Difficulty: req.Difficulty}
	if err := this.db.Rating.Put(ctx, req.PuzzleID, user.ID, rating); err != nil {
		return nil, st.StackTrace(this.db.Rating.Put, err)
	}

	stats, err := this.db.Rating.Stats(ctx, req.PuzzleID)
	if err != nil {
		return nil, st.StackTrace(this.db.Rating.Stats, err)
	}

	if err := this.db.Puzzle.SetRatingStats(ctx, req.PuzzleID, stats); err != nil {
		return nil, st.StackTrace(this.db.Puzzle.SetRatingStats, err)
	}

	mine, err := this.db.Rating.Find(ctx, req.PuzzleID, user.ID)
	if err != nil {
		return nil, st.StackTrace(this.db.Rating.Find, err)
	}

	view := model.NewRatingView(stats, mine)

	return &view, nil
}
