package list

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Sili-Sudoku/api/libs/cursor"
	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/history/model"
)

const (
	defaultLimit = 20
	maxLimit     = 50
)

type EndpointReq struct {
	rawLimit  string
	rawCursor string

	limit int
	after *model.Cursor
}

type EndpointResp struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"nextCursor"`
}

type Item struct {
	PuzzleID  string     `json:"puzzleId"`
	Title     *string    `json:"title"` // only for a published puzzle that still exists
	Seconds   *int       `json:"seconds"`
	SolvedAt  *time.Time `json:"solvedAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

func (this *EndpointReq) BindQuery(req *http.Request) {
	q := req.URL.Query()
	this.rawLimit, this.rawCursor = q.Get("limit"), q.Get("cursor")
}

func (this *EndpointReq) Validate() error {
	this.limit = defaultLimit
	if this.rawLimit != "" {
		n, err := strconv.Atoi(this.rawLimit)
		if err != nil || n < 1 || n > maxLimit {
			return http_outcome.NewErrorWithCode(http.StatusBadRequest, "bad_limit", errors.New("limit must be 1 to "+strconv.Itoa(maxLimit)))
		}
		this.limit = n
	}
	if this.rawCursor != "" {
		after, ok := cursor.Decode[model.Cursor](this.rawCursor)
		if !ok || after.PuzzleID == "" {
			return http_outcome.NewErrorWithCode(http.StatusBadRequest, "bad_cursor", errors.New("cursor is not valid"))
		}
		this.after = after
	}

	return nil
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	userID, err := this.auth.GetUserID(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUserID, err)
	}

	entries, err := this.db.SolveHistory.List(ctx, userID, req.after, req.limit+1)
	if err != nil {
		return nil, st.StackTrace(this.db.SolveHistory.List, err)
	}

	resp = &EndpointResp{Items: make([]Item, 0, len(entries))}
	if len(entries) > req.limit {
		entries = entries[:req.limit]
		last := entries[req.limit-1]
		resp.NextCursor = cursor.Encode(&model.Cursor{UpdatedAt: last.UpdatedAt, PuzzleID: last.PuzzleID})
	}
	for _, e := range entries {
		item := Item{PuzzleID: e.PuzzleID, Seconds: e.Seconds, SolvedAt: e.SolvedAt, UpdatedAt: e.UpdatedAt}
		// A missing, hidden or unreadable puzzle only costs the title: the user's own time is still listed.
		if p, err := this.db.Puzzle.Find(ctx, e.PuzzleID); err == nil && p != nil && p.Visible() {
			item.Title = &p.Title
		}
		resp.Items = append(resp.Items, item)
	}

	return resp, nil
}
