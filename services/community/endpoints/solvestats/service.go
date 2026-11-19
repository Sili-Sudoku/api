package solvestats

import (
	"context"
	"errors"
	"net/http"
	"sort"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
)

type EndpointReq struct {
	PuzzleID string
}

// EndpointResp is how many people solved a puzzle and the median time. MedianSeconds is null without solves.
type EndpointResp struct {
	Solves        int  `json:"solves"`
	MedianSeconds *int `json:"medianSeconds"`
}

func (this *EndpointReq) BindPath(req *http.Request) {
	this.PuzzleID = req.PathValue("id")
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	puzzle, err := this.db.Puzzle.Find(ctx, req.PuzzleID)
	if err != nil {
		return nil, st.StackTrace(this.db.Puzzle.Find, err)
	}

	if puzzle == nil || !puzzle.Visible() {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	}

	seconds, err := this.db.SolveHistory.SolveSeconds(ctx, req.PuzzleID)
	if err != nil {
		return nil, st.StackTrace(this.db.SolveHistory.SolveSeconds, err)
	}

	if len(seconds) == 0 {
		return &EndpointResp{}, nil
	}

	sort.Ints(seconds)
	median := seconds[len(seconds)/2]

	return &EndpointResp{Solves: len(seconds), MedianSeconds: &median}, nil
}
