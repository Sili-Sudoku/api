package listcomments

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/Sili-Sudoku/api/libs/cursor"
	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/community/model"
)

const (
	defaultLimit = 20
	maxLimit     = 50
)

type EndpointReq struct {
	PuzzleID  string
	rawLimit  string
	rawCursor string

	limit int
	after *model.CommentCursor
}

type EndpointResp struct {
	Items      []model.CommentView `json:"items"`
	NextCursor string              `json:"nextCursor"`
}

func (this *EndpointReq) BindPath(req *http.Request) {
	this.PuzzleID = req.PathValue("id")
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
		after, ok := cursor.Decode[model.CommentCursor](this.rawCursor)
		if !ok || after.ID <= 0 {
			return http_outcome.NewErrorWithCode(http.StatusBadRequest, "bad_cursor", errors.New("cursor is not valid"))
		}
		this.after = after
	}

	return nil
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

	comments, err := this.db.Comment.List(ctx, req.PuzzleID, req.after, req.limit+1)
	if err != nil {
		return nil, st.StackTrace(this.db.Comment.List, err)
	}

	resp = &EndpointResp{Items: make([]model.CommentView, 0, len(comments))}
	if len(comments) > req.limit {
		comments = comments[:req.limit]
		last := comments[req.limit-1]
		resp.NextCursor = cursor.Encode(&model.CommentCursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}

	ids := make([]int64, 0, len(comments))
	for _, c := range comments {
		ids = append(ids, c.UserID)
	}
	names, err := this.db.User.Usernames(ctx, ids)
	if err != nil {
		return nil, st.StackTrace(this.db.User.Usernames, err)
	}

	var userID int64
	if user != nil {
		userID = user.ID
	}
	for _, c := range comments {
		resp.Items = append(resp.Items, model.NewCommentView(c, names, userID))
	}

	return resp, nil
}
