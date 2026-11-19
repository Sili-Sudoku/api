package record

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/history/model"
)

type EndpointReq struct {
	Seconds  *int       `json:"seconds"`
	SolvedAt *time.Time `json:"solvedAt"`

	PuzzleID string `json:"-"`
}

type EndpointResp struct {
	PuzzleID  string     `json:"puzzleId"`
	Title     *string    `json:"title"` // always null here; the list fills it in
	Seconds   *int       `json:"seconds"`
	SolvedAt  *time.Time `json:"solvedAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

func (this *EndpointReq) BindPath(req *http.Request) {
	this.PuzzleID = req.PathValue("puzzleId")
}

func (this *EndpointReq) Validate() error {
	if !model.IDPattern.MatchString(this.PuzzleID) || (this.Seconds != nil && (*this.Seconds < 1 || *this.Seconds > model.MaxSeconds)) {
		return errors.New("bad puzzle id, time or date")
	}
	if this.SolvedAt != nil && this.SolvedAt.After(time.Now().Add(24*time.Hour)) {
		return errors.New("bad puzzle id, time or date")
	}

	return nil
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	userID, err := this.auth.GetUserID(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUserID, err)
	}

	e, err := this.db.SolveHistory.Upsert(ctx, userID, model.Entry{PuzzleID: req.PuzzleID, SolvedAt: req.SolvedAt, Seconds: req.Seconds})
	if errors.Is(err, model.ErrFull) {
		return nil, http_outcome.NewErrorWithCode(http.StatusConflict, "history_full", errors.New("history is full; delete some of it first"))
	}
	if err != nil {
		return nil, st.StackTrace(this.db.SolveHistory.Upsert, err)
	}

	return &EndpointResp{PuzzleID: e.PuzzleID, Seconds: e.Seconds, SolvedAt: e.SolvedAt, UpdatedAt: e.UpdatedAt}, nil
}
