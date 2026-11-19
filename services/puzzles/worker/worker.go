// Package worker validates pending puzzles with the engine. The publish endpoint starts it in the background; it never
// runs inside a request.
package worker

import (
	"context"
	"fmt"

	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/libs/validate"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Worker struct {
	db DB
	v  validate.Validator
}

func New(db DB, v validate.Validator) *Worker { return &Worker{db: db, v: v} }

type DB interface {
	Find(ctx context.Context, id string) (*model.Puzzle, error)
	PendingIDs(ctx context.Context) ([]string, error)
	SetVerdict(ctx context.Context, id string, status model.Status, v validate.Verdict) error
}

// Validate runs the engine on a pending puzzle and sets its final status. A failure of the engine leaves the puzzle
// pending, for the next sweep.
func (this *Worker) Validate(ctx context.Context, id string) error {
	p, err := this.db.Find(ctx, id)
	if err != nil {
		return st.StackTrace(this.db.Find, err)
	}

	if p == nil {
		return model.ErrNotFound
	}
	if p.Status != model.Pending {
		return nil
	}
	verdict, err := this.v.Validate(ctx, p.Body)
	if err != nil {
		return st.StackTrace(this.v.Validate, err)
	}

	status := model.Rejected
	if verdict.Valid {
		status = model.Published
	}
	if err := this.db.SetVerdict(ctx, id, status, verdict); err != nil {
		return st.StackTrace(this.db.SetVerdict, err)
	}

	return nil
}

// ValidatePending validates every puzzle that is still pending (after a restart, or a tool failure). It returns the
// first error after trying all of them.
func (this *Worker) ValidatePending(ctx context.Context) error {
	ids, err := this.db.PendingIDs(ctx)
	if err != nil {
		return st.StackTrace(this.db.PendingIDs, err)
	}

	var first error
	for _, id := range ids {
		if err := this.Validate(ctx, id); err != nil && first == nil {
			first = fmt.Errorf("validate %s: %w", id, err)
		}
	}

	return first
}
