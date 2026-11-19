package feed

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/Sili-Sudoku/api/libs/cursor"
	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/community/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

const (
	defaultLimit = 20
	maxLimit     = 50
)

type EndpointReq struct {
	rawLimit  string
	rawCursor string

	limit int
	after *puzzles.Cursor
}

type EndpointResp struct {
	Items      []puzzles.Summary `json:"items"`
	NextCursor string            `json:"nextCursor"`
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
		after, ok := cursor.Decode[puzzles.Cursor](this.rawCursor)
		if !ok || !after.Valid() {
			return http_outcome.NewErrorWithCode(http.StatusBadRequest, "bad_cursor", errors.New("cursor is not valid"))
		}
		this.after = after
	}

	return nil
}

// Execute lists the newest puzzles of the creators that the user follows.
func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	me, err := this.auth.GetUser(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUser, err)
	}

	creatorIDs, err := this.db.Follow.List(ctx, me.ID, model.MaxFollowing)
	if err != nil {
		return nil, st.StackTrace(this.db.Follow.List, err)
	}

	resp = &EndpointResp{Items: []puzzles.Summary{}}
	if len(creatorIDs) == 0 {
		return resp, nil
	}

	items, err := this.db.Puzzle.List(ctx, puzzles.Filter{OwnerIDs: creatorIDs, Limit: req.limit + 1, After: req.after})
	if err != nil {
		return nil, st.StackTrace(this.db.Puzzle.List, err)
	}

	if len(items) > req.limit {
		items = items[:req.limit]
		last := items[req.limit-1]
		resp.NextCursor = cursor.Encode(&puzzles.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}

	var ownerIDs []int64
	for _, p := range items {
		if p.OwnerID != 0 {
			ownerIDs = append(ownerIDs, p.OwnerID)
		}
	}
	names, err := this.db.User.Usernames(ctx, ownerIDs)
	if err != nil {
		return nil, st.StackTrace(this.db.User.Usernames, err)
	}

	for _, p := range items {
		resp.Items = append(resp.Items, puzzles.NewSummary(p, names))
	}

	return resp, nil
}
