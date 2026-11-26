package getpuzzle

import (
	"context"
	"errors"
	"net/http"
	"testing"

	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	"github.com/Sili-Sudoku/api/libs/validate"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

// user is a signed-in user with this id, or nobody for 0.
func user(id int64) *mp.OptionalAuth {
	return &mp.OptionalAuth{FindUserFunc: func(ctx context.Context) *accounts.User {
		if id == 0 {
			return nil
		}
		return &accounts.User{ID: id}
	}}
}

func finds(t *testing.T, p *model.Puzzle, err error) *mp.Puzzle {
	return &mp.Puzzle{T: t, FindFunc: func(ctx context.Context, id string) (*model.Puzzle, error) { return p, err }}
}

func names(t *testing.T, m map[int64]string, err error) *mp.User {
	return &mp.User{T: t, UsernamesFunc: func(ctx context.Context, ids []int64) (map[int64]string, error) { return m, err }}
}

func TestNew(t *testing.T) {
	auth, db := &mp.OptionalAuth{T: t}, &DB{}
	v := &mp.Validator{T: t}
	if ep := New(auth, db, v); ep.auth != auth || ep.db != db || ep.validator != v {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_BindPath(t *testing.T) {
	mp.RunBindPathTests(t, []mp.BindPathTest{
		{Name: "id", PathValues: map[string]string{"id": "p1"}, Input: &EndpointReq{}, Want: &EndpointReq{ID: "p1"}},
	})
}

func TestEndpoint_Execute(t *testing.T) {
	body := []byte(`{"cells":1}`)
	ann := "ann"
	published := &model.Puzzle{ID: "p1", Body: body, Status: model.Published, Title: "T", OwnerID: 7,
		Verdict: &validate.Verdict{Valid: true, Solutions: "one"}, Tags: []string{}, Variants: []string{}}
	summary := model.NewSummary(published, map[int64]string{7: ann})
	rejected := &model.Puzzle{ID: "p2", Body: body, Status: model.Rejected, OwnerID: 7, Verdict: &validate.Verdict{Solutions: "none"}}
	rejectedSummary := model.NewSummary(rejected, map[int64]string{7: ann})
	many := &model.Puzzle{ID: "p3", Body: body, Status: model.Published, Verdict: &validate.Verdict{Valid: true, Solutions: "many"}}
	manySummary := model.NewSummary(many, nil)

	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name:     "db.Puzzle.Find_error",
			Endpoint: &Endpoint{auth: user(0), db: &DB{Puzzle: finds(t, nil, errors.New("dbErr"))}},
			Input:    &EndpointReq{ID: "p1"},
			WantErr:  true,
		},
		{
			Name:       "db.Puzzle.Find_missing",
			Endpoint:   &Endpoint{auth: user(0), db: &DB{Puzzle: finds(t, nil, nil)}},
			Input:      &EndpointReq{ID: "p1"},
			WantErr:    true,
			WantStatus: http.StatusNotFound,
			WantCode:   "not_found",
		},
		{
			Name:     "pending_has_no_body_and_no_summary",
			Endpoint: &Endpoint{auth: user(7), db: &DB{Puzzle: finds(t, &model.Puzzle{ID: "p1", Body: body, Status: model.Pending, OwnerID: 7}, nil)}},
			Input:    &EndpointReq{ID: "p1"},
			WantResp: &EndpointResp{ID: "p1", Status: model.Pending, Mine: true},
		},
		{
			Name:     "rejected_has_a_body_for_its_owner_only",
			Endpoint: &Endpoint{auth: user(8), db: &DB{Puzzle: finds(t, rejected, nil), User: names(t, map[int64]string{7: ann}, nil)}},
			Input:    &EndpointReq{ID: "p2"},
			WantResp: &EndpointResp{ID: "p2", Status: model.Rejected, Verdict: rejected.Verdict, Summary: &rejectedSummary},
		},
		{
			Name:     "rejected_owner",
			Endpoint: &Endpoint{auth: user(7), db: &DB{Puzzle: finds(t, rejected, nil), User: names(t, map[int64]string{7: ann}, nil)}},
			Input:    &EndpointReq{ID: "p2"},
			WantResp: &EndpointResp{ID: "p2", Status: model.Rejected, Verdict: rejected.Verdict, Puzzle: body, Summary: &rejectedSummary, Mine: true},
		},
		{
			Name:     "db.User.Usernames_error_still_serves_the_puzzle",
			Endpoint: &Endpoint{auth: user(0), db: &DB{Puzzle: finds(t, many, nil), User: names(t, nil, errors.New("dbErr"))}},
			Input:    &EndpointReq{ID: "p3"},
			WantResp: &EndpointResp{ID: "p3", Status: model.Published, Verdict: many.Verdict, Puzzle: body, Summary: &manySummary, Warnings: []string{"many_solutions"}},
		},
		{
			Name:     "success_published_signed_out",
			Endpoint: &Endpoint{auth: user(0), db: &DB{Puzzle: finds(t, published, nil), User: names(t, map[int64]string{7: ann}, nil)}},
			Input:    &EndpointReq{ID: "p1"},
			WantResp: &EndpointResp{ID: "p1", Status: model.Published, Verdict: published.Verdict, Puzzle: body, Summary: &summary},
		},
	})
}

func TestEndpoint_Execute_RetriesAPendingPuzzle(t *testing.T) {
	body := []byte(`{"cells":1}`)
	pending := &model.Puzzle{ID: "p1", Body: body, Status: model.Pending}
	done := &model.Puzzle{ID: "p1", Body: body, Status: model.Published, Verdict: &validate.Verdict{Valid: true, Solutions: "one"}}

	t.Run("it_is_published_by_the_retry", func(t *testing.T) {
		calls := 0
		db := &mp.Puzzle{T: t, FindFunc: func(context.Context, string) (*model.Puzzle, error) {
			calls++
			if calls == 1 {
				return pending, nil
			}
			return done, nil
		}}
		v := &mp.Validator{T: t, ValidateFunc: func(context.Context, string) error { return nil }}
		resp, err := (&Endpoint{auth: user(0), db: &DB{Puzzle: db, User: names(t, nil, nil)}, validator: v}).Execute(context.Background(), &EndpointReq{ID: "p1"})
		if err != nil || resp.Status != model.Published || resp.Puzzle == nil {
			t.Fatalf("resp = %+v, err = %v", resp, err)
		}
	})

	t.Run("an_engine_failure_leaves_it_pending", func(t *testing.T) {
		v := &mp.Validator{T: t, ValidateFunc: func(context.Context, string) error { return errors.New("engineErr") }}
		resp, err := (&Endpoint{auth: user(0), db: &DB{Puzzle: finds(t, pending, nil)}, validator: v}).Execute(context.Background(), &EndpointReq{ID: "p1"})
		if err != nil || resp.Status != model.Pending || resp.Puzzle != nil {
			t.Fatalf("resp = %+v, err = %v", resp, err)
		}
	})
}
