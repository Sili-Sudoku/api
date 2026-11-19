package getpuzzle

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/libs/validate"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

type EndpointReq struct {
	ID string
}

type EndpointResp struct {
	ID       string            `json:"id"`
	Status   model.Status      `json:"status"`
	Verdict  *validate.Verdict `json:"verdict"`
	Puzzle   json.RawMessage   `json:"puzzle,omitempty"`
	Summary  *model.Summary    `json:"summary,omitempty"`
	Warnings []string          `json:"warnings,omitempty"`
	Mine     bool              `json:"mine"`
}

func (this *EndpointReq) BindPath(req *http.Request) {
	this.ID = req.PathValue("id")
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	p, err := this.db.Puzzle.Find(ctx, req.ID)
	if err != nil {
		return nil, st.StackTrace(this.db.Puzzle.Find, err)
	}

	if p == nil {
		return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
	}

	user := this.auth.FindUser(ctx)
	isOwner := user != nil && p.OwnerID == user.ID
	resp = &EndpointResp{ID: p.ID, Status: p.Status, Verdict: p.Verdict, Mine: isOwner}
	// A puzzle that is not published has no public body: it may be bad, or not checked yet.
	if p.Status == model.Published || (isOwner && p.Status != model.Pending) {
		resp.Puzzle = p.Body
	}
	if p.Status != model.Pending {
		summary := model.NewSummary(p, this.ownerNames(ctx, p))
		resp.Summary = &summary
	}
	if p.Verdict != nil && p.Verdict.Valid && p.Verdict.Solutions == "many" {
		resp.Warnings = []string{"many_solutions"}
	}

	return resp, nil
}

// ownerNames only costs the owner's name when it fails: the puzzle is still served.
func (this *Endpoint) ownerNames(ctx context.Context, p *model.Puzzle) map[int64]string {
	if p.OwnerID == 0 {
		return nil
	}
	names, err := this.db.User.Usernames(ctx, []int64{p.OwnerID})
	if err != nil {
		log.Printf("usernames: %v", err)
	}

	return names
}
