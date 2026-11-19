package list

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/Sili-Sudoku/api/libs/cursor"
	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

const (
	defaultLimit = 20
	maxLimit     = 50
)

type EndpointReq struct {
	rawAuthor, rawCursor, rawLimit, rawSort string
	filter                                  model.Filter
}

type EndpointResp struct {
	Items      []model.Summary `json:"items"`
	NextCursor string          `json:"nextCursor"`
}

func (this *EndpointReq) BindQuery(req *http.Request) {
	q := req.URL.Query()
	this.rawAuthor, this.rawCursor, this.rawLimit, this.rawSort = q.Get("author"), q.Get("cursor"), q.Get("limit"), q.Get("sort")
	this.filter = model.Filter{Tag: q.Get("tag"), Variant: q.Get("variant"), Difficulty: q.Get("difficulty")}
}

// Validate reads the sort before the cursor: a cursor of the top order is an offset, of the newest order a position.
func (this *EndpointReq) Validate() error {
	this.filter.Limit = defaultLimit
	if this.rawLimit != "" {
		n, err := strconv.Atoi(this.rawLimit)
		if err != nil || n < 1 || n > maxLimit {
			return http_outcome.NewErrorWithCode(http.StatusBadRequest, "bad_limit", errors.New("limit must be 1 to "+strconv.Itoa(maxLimit)))
		}
		this.filter.Limit = n
	}
	switch this.rawSort {
	case "", "newest":
	case "top":
		this.filter.Top = true
	default:
		return http_outcome.NewErrorWithCode(http.StatusBadRequest, "bad_sort", errors.New("sort must be newest or top"))
	}
	if this.rawCursor != "" {
		c, ok := cursor.Decode[model.Cursor](this.rawCursor)
		if !ok || !c.Valid() {
			return http_outcome.NewErrorWithCode(http.StatusBadRequest, "bad_cursor", errors.New("cursor is not valid"))
		}
		if this.filter.Top {
			this.filter.Offset = c.Offset
		} else {
			this.filter.After = c
		}
	}

	return nil
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	f := req.filter
	if req.rawAuthor != "" {
		author, err := this.db.User.FindByName(ctx, req.rawAuthor)
		if err != nil {
			return nil, st.StackTrace(this.db.User.FindByName, err)
		}

		if author == nil {
			return &EndpointResp{Items: []model.Summary{}}, nil
		}
		f.OwnerIDs = []int64{author.ID}
	}

	f.Limit++ // one more than the page, to know whether there is a next page
	items, err := this.db.Puzzle.List(ctx, f)
	if err != nil {
		return nil, st.StackTrace(this.db.Puzzle.List, err)
	}

	resp = &EndpointResp{}
	if len(items) > req.filter.Limit {
		items = items[:req.filter.Limit]
		if f.Top {
			resp.NextCursor = cursor.Encode(model.Cursor{Offset: f.Offset + req.filter.Limit})
		} else {
			last := items[len(items)-1]
			resp.NextCursor = cursor.Encode(model.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
		}
	}

	names := this.ownerNames(ctx, items)
	resp.Items = make([]model.Summary, 0, len(items))
	for _, p := range items {
		resp.Items = append(resp.Items, model.NewSummary(p, names))
	}

	return resp, nil
}

// ownerNames only costs the owners' names when it fails: the page is still served.
func (this *Endpoint) ownerNames(ctx context.Context, items []*model.Puzzle) map[int64]string {
	var ids []int64
	for _, p := range items {
		if p.OwnerID != 0 {
			ids = append(ids, p.OwnerID)
		}
	}
	names, err := this.db.User.Usernames(ctx, ids)
	if err != nil {
		log.Printf("usernames: %v", err)
	}

	return names
}
