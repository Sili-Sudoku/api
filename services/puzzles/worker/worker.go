// Package worker validates pending puzzles with the engine. The publish endpoint runs it inside its own request, and a
// read of a puzzle that is still pending runs it again, so nothing runs between requests: the host (Cloud Run) only has
// to give CPU to requests, and the program can scale to zero.
package worker

import (
	"context"
	"fmt"
	"sync"

	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/libs/validate"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

// MaxConcurrent bounds the engine processes one program runs at once. Each is a Node process of up to about 110 MB, so
// more would use up the memory of a small instance. A request that cannot get a slot waits for one until its context ends.
const MaxConcurrent = 3

type Worker struct {
	db DB
	v  validate.Validator

	slots   chan struct{}
	mu      sync.Mutex
	running map[string]bool
}

func New(db DB, v validate.Validator) *Worker {
	return &Worker{db: db, v: v, slots: make(chan struct{}, MaxConcurrent), running: map[string]bool{}}
}

// begin says if the caller may validate id: only one run per puzzle at a time.
func (this *Worker) begin(id string) bool {
	this.mu.Lock()
	defer this.mu.Unlock()

	if this.running[id] {
		return false
	}

	this.running[id] = true

	return true
}

func (this *Worker) end(id string) {
	this.mu.Lock()
	defer this.mu.Unlock()

	delete(this.running, id)
}

type DB interface {
	Find(ctx context.Context, id string) (*model.Puzzle, error)
	PendingIDs(ctx context.Context) ([]string, error)
	SetVerdict(ctx context.Context, id string, status model.Status, v validate.Verdict) error
}

// Validate runs the engine on a pending puzzle and sets its final status. A failure of the engine leaves the puzzle
// pending, and the next read of it tries again. If another call is already validating id, this one returns at once and
// that call finishes the job, so a flood of reads of one pending puzzle runs the engine once.
func (this *Worker) Validate(ctx context.Context, id string) error {
	if !this.begin(id) {
		return nil
	}

	defer this.end(id)

	select {
	case this.slots <- struct{}{}:
		defer func() { <-this.slots }()
	case <-ctx.Done():
		return ctx.Err()
	}

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

// ValidatePending validates every puzzle that is still pending (the seed tool and tests use it; the programs do not loop
// on it). It returns the first error after trying all of them.
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
