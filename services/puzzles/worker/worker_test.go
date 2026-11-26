package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	"github.com/Sili-Sudoku/api/libs/validate"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

type engine func(ctx context.Context, puzzle []byte) (validate.Verdict, error)

func (f engine) Validate(ctx context.Context, puzzle []byte) (validate.Verdict, error) {
	return f(ctx, puzzle)
}

func TestNew(t *testing.T) {
	db := &mp.Puzzle{T: t}
	if w := New(db, validate.Accept{}); w.db != db {
		t.Fatalf("New did not keep its dependencies: %+v", w)
	}
}

func TestWorker_Validate(t *testing.T) {
	pending := &model.Puzzle{ID: "a", Body: []byte(`{}`), Status: model.Pending}
	finds := func(p *model.Puzzle, err error) func(context.Context, string) (*model.Puzzle, error) {
		return func(context.Context, string) (*model.Puzzle, error) { return p, err }
	}
	tests := []struct {
		Name       string
		Find       func(context.Context, string) (*model.Puzzle, error)
		Engine     engine
		WantErr    error
		WantAnyErr bool
		WantStatus model.Status // "" = SetVerdict must not be called
	}{
		{Name: "db.Find_error", Find: finds(nil, errors.New("dbErr")), WantAnyErr: true},
		{Name: "db.Find_missing", Find: finds(nil, nil), WantErr: model.ErrNotFound},
		{Name: "not_pending", Find: finds(&model.Puzzle{ID: "a", Status: model.Published}, nil)},
		{
			Name: "engine_error_stays_pending",
			Find: finds(pending, nil),
			Engine: func(context.Context, []byte) (validate.Verdict, error) {
				return validate.Verdict{}, errors.New("engineErr")
			},
			WantAnyErr: true,
		},
		{
			Name: "valid_is_published",
			Find: finds(pending, nil),
			Engine: func(context.Context, []byte) (validate.Verdict, error) {
				return validate.Verdict{Valid: true, Solutions: "one"}, nil
			},
			WantStatus: model.Published,
		},
		{
			Name: "invalid_is_rejected",
			Find: finds(pending, nil),
			Engine: func(context.Context, []byte) (validate.Verdict, error) {
				return validate.Verdict{Solutions: "none"}, nil
			},
			WantStatus: model.Rejected,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			var got model.Status
			db := &mp.Puzzle{T: t, FindFunc: tt.Find}
			if tt.WantStatus != "" {
				db.SetVerdictFunc = func(_ context.Context, _ string, status model.Status, _ validate.Verdict) error {
					got = status
					return nil
				}
			}
			err := New(db, tt.Engine).Validate(context.Background(), "a")
			if tt.WantErr != nil && !errors.Is(err, tt.WantErr) || (err != nil) != (tt.WantErr != nil || tt.WantAnyErr) {
				t.Fatalf("err = %v", err)
			}
			if got != tt.WantStatus {
				t.Fatalf("status = %q, want %q", got, tt.WantStatus)
			}
		})
	}

	t.Run("db.SetVerdict_error", func(t *testing.T) {
		db := &mp.Puzzle{T: t, FindFunc: finds(pending, nil), SetVerdictFunc: func(context.Context, string, model.Status, validate.Verdict) error {
			return errors.New("dbErr")
		}}
		if err := New(db, validate.Accept{}).Validate(context.Background(), "a"); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestWorker_ValidatePending(t *testing.T) {
	t.Run("db.PendingIDs_error", func(t *testing.T) {
		db := &mp.Puzzle{T: t, PendingIDsFunc: func(context.Context) ([]string, error) { return nil, errors.New("dbErr") }}
		if err := New(db, validate.Accept{}).ValidatePending(context.Background()); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("tries_all_and_returns_the_first_error", func(t *testing.T) {
		var verdicts []string
		db := &mp.Puzzle{
			T:              t,
			PendingIDsFunc: func(context.Context) ([]string, error) { return []string{"a", "b", "c"}, nil },
			FindFunc: func(_ context.Context, id string) (*model.Puzzle, error) {
				return &model.Puzzle{ID: id, Status: model.Pending}, nil
			},
			SetVerdictFunc: func(_ context.Context, id string, _ model.Status, _ validate.Verdict) error {
				verdicts = append(verdicts, id)
				return nil
			},
		}
		fail := engine(func(context.Context, []byte) (validate.Verdict, error) {
			return validate.Verdict{}, errors.New("engineErr")
		})
		calls := 0
		w := New(db, engine(func(ctx context.Context, b []byte) (validate.Verdict, error) {
			calls++
			if calls == 1 {
				return fail(ctx, b)
			}
			return validate.Verdict{Valid: true}, nil
		}))
		err := w.ValidatePending(context.Background())
		if err == nil || err.Error()[:len("validate a:")] != "validate a:" {
			t.Fatalf("err = %v", err)
		}
		if len(verdicts) != 2 || verdicts[0] != "b" || verdicts[1] != "c" {
			t.Fatalf("verdicts = %v", verdicts)
		}
	})
}

func TestWorker_Validate_OneRunPerPuzzle(t *testing.T) {
	pending := &model.Puzzle{ID: "a", Body: []byte(`{}`), Status: model.Pending}
	db := &mp.Puzzle{T: t,
		FindFunc:       func(context.Context, string) (*model.Puzzle, error) { return pending, nil },
		SetVerdictFunc: func(context.Context, string, model.Status, validate.Verdict) error { return nil }}
	started, release := make(chan struct{}), make(chan struct{})
	var runs atomic.Int32
	w := New(db, engine(func(context.Context, []byte) (validate.Verdict, error) {
		runs.Add(1)
		started <- struct{}{}
		<-release
		return validate.Verdict{Valid: true, Solutions: "one"}, nil
	}))

	done := make(chan error)
	go func() { done <- w.Validate(context.Background(), "a") }()
	<-started
	if err := w.Validate(context.Background(), "a"); err != nil { // returns at once, does not run the engine
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil || runs.Load() != 1 {
		t.Fatalf("err = %v, runs = %d", err, runs.Load())
	}
}

func TestWorker_Validate_AtMostMaxConcurrent(t *testing.T) {
	db := &mp.Puzzle{T: t,
		FindFunc: func(_ context.Context, id string) (*model.Puzzle, error) {
			return &model.Puzzle{ID: id, Body: []byte(`{}`), Status: model.Pending}, nil
		},
		SetVerdictFunc: func(context.Context, string, model.Status, validate.Verdict) error { return nil }}
	var now, peak atomic.Int32
	release := make(chan struct{})
	w := New(db, engine(func(context.Context, []byte) (validate.Verdict, error) {
		n := now.Add(1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		<-release
		now.Add(-1)
		return validate.Verdict{Valid: true, Solutions: "one"}, nil
	}))

	var wg sync.WaitGroup
	for i := 0; i < MaxConcurrent+4; i++ {
		wg.Add(1)
		go func(id string) { defer wg.Done(); _ = w.Validate(context.Background(), id) }(string(rune('a' + i)))
	}
	for now.Load() < MaxConcurrent {
	}
	close(release)
	wg.Wait()
	if peak.Load() != MaxConcurrent {
		t.Fatalf("peak = %d, want %d", peak.Load(), MaxConcurrent)
	}
}

func TestWorker_Validate_WaitsForASlotUntilTheContextEnds(t *testing.T) {
	w := New(&mp.Puzzle{T: t}, validate.Accept{})
	for i := 0; i < MaxConcurrent; i++ {
		w.slots <- struct{}{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := w.Validate(ctx, "a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}
