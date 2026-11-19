package postcomment

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/community/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

type EndpointReq struct {
	Text string `json:"text"`

	PuzzleID string `json:"-"`
}

type EndpointResp struct {
	model.CommentView
}

func (this *EndpointResp) StatusCode() int { return http.StatusCreated }

func (this *EndpointReq) BindPath(req *http.Request) {
	this.PuzzleID = req.PathValue("id")
}

// Validate trims the text; it must hold 1 to MaxComment characters of plain text.
func (this *EndpointReq) Validate() error {
	this.Text = strings.TrimSpace(this.Text)
	if this.Text == "" || utf8.RuneCountInString(this.Text) > model.MaxComment || !utf8.ValidString(this.Text) || strings.ContainsRune(this.Text, 0) {
		return errors.New("stars and difficulty are 1 to 5; a comment has 1 to 2000 characters")
	}

	return nil
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	user, err := this.auth.GetUser(ctx)
	if err != nil {
		return nil, st.StackTrace(this.auth.GetUser, err)
	}

	if !this.limit.Allow("user:" + user.Username) {
		return nil, http_outcome.NewErrorWithCode(http.StatusTooManyRequests, "rate_limited", errors.New("too many comments; try again later"))
	}

	puzzle, err := this.db.Puzzle.Find(ctx, req.PuzzleID)
	if err != nil {
		return nil, st.StackTrace(this.db.Puzzle.Find, err)
	}

	if puzzle == nil || !puzzle.Visible() {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	}

	comment := &model.Comment{PuzzleID: req.PuzzleID, UserID: user.ID, Body: req.Text, Moderation: puzzles.ModNone}
	if err := this.db.Comment.Create(ctx, comment); err != nil {
		return nil, st.StackTrace(this.db.Comment.Create, err)
	}

	names := map[int64]string{user.ID: user.Username}

	return &EndpointResp{CommentView: model.NewCommentView(comment, names, user.ID)}, nil
}
