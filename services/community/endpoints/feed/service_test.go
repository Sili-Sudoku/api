package feed

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sili-Sudoku/api/libs/cursor"
	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/community/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

func okAuth(t *testing.T) *mp.UserAuth {
	return &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*accounts.User, error) { return &accounts.User{ID: 7}, nil }}
}

func followsOf(t *testing.T, ids []int64, err error) *mp.Follow {
	return &mp.Follow{T: t, ListFunc: func(ctx context.Context, followerID int64, limit int) ([]int64, error) {
		if followerID != 7 || limit != model.MaxFollowing {
			t.Errorf("List(%d, %d)", followerID, limit)
		}
		return ids, err
	}}
}

func TestNew(t *testing.T) {
	auth, db := &mp.UserAuth{T: t}, &DB{}
	if ep := New(auth, db); ep.auth != auth || ep.db != db {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_Validate(t *testing.T) {
	mp.RunValidateTests(t, []mp.ValidateTest{
		{Name: "limit is not a number", Input: &EndpointReq{rawLimit: "x"}, WantErr: true},
		{Name: "limit is zero", Input: &EndpointReq{rawLimit: "0"}, WantErr: true},
		{Name: "limit is too big", Input: &EndpointReq{rawLimit: "51"}, WantErr: true},
		{Name: "cursor is not a cursor", Input: &EndpointReq{rawCursor: "zzz"}, WantErr: true},
		{Name: "cursor without an id or an offset", Input: &EndpointReq{rawCursor: cursor.Encode(&puzzles.Cursor{})}, WantErr: true},
		{Name: "defaults", Input: &EndpointReq{}},
		{Name: "limit and cursor", Input: &EndpointReq{rawLimit: "50", rawCursor: cursor.Encode(&puzzles.Cursor{ID: "p1"})}},
	})
}

func TestEndpoint_Execute(t *testing.T) {
	t0 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	items := []*puzzles.Puzzle{
		{ID: "p2", Title: "two", OwnerID: 3, CreatedAt: t0.Add(time.Hour), Tags: []string{}, Variants: []string{}},
		{ID: "p1", Title: "one", OwnerID: 3, CreatedAt: t0, Tags: []string{}, Variants: []string{}},
	}
	puzzlesOf := func(ps []*puzzles.Puzzle, err error) *mp.Puzzle {
		return &mp.Puzzle{T: t, ListFunc: func(ctx context.Context, f puzzles.Filter) ([]*puzzles.Puzzle, error) {
			if len(f.OwnerIDs) != 2 || f.Limit != 3 || f.Top {
				t.Errorf("List(%+v)", f)
			}
			return ps, err
		}}
	}
	usersOf := func(names map[int64]string, err error) *mp.User {
		return &mp.User{T: t, UsernamesFunc: func(ctx context.Context, ids []int64) (map[int64]string, error) { return names, err }}
	}
	maker := "maker"

	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name: "auth.GetUser_error",
			Endpoint: &Endpoint{
				auth: &mp.UserAuth{T: t, GetUserFunc: func(ctx context.Context) (*accounts.User, error) { return nil, errors.New("authErr") }},
				db:   &DB{},
			},
			Input:   &EndpointReq{limit: 2},
			WantErr: true,
		},
		{
			Name:     "db.Follow.List_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{Follow: followsOf(t, nil, errors.New("dbErr"))}},
			Input:    &EndpointReq{limit: 2},
			WantErr:  true,
		},
		{
			Name:     "success_follows_nobody",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{Follow: followsOf(t, nil, nil)}},
			Input:    &EndpointReq{limit: 2},
			WantResp: &EndpointResp{Items: []puzzles.Summary{}},
		},
		{
			Name:     "db.Puzzle.List_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{Follow: followsOf(t, []int64{3, 4}, nil), Puzzle: puzzlesOf(nil, errors.New("dbErr"))}},
			Input:    &EndpointReq{limit: 2},
			WantErr:  true,
		},
		{
			Name: "db.User.Usernames_error",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{
				Follow: followsOf(t, []int64{3, 4}, nil), Puzzle: puzzlesOf(items, nil), User: usersOf(nil, errors.New("dbErr")),
			}},
			Input:   &EndpointReq{limit: 2},
			WantErr: true,
		},
		{
			Name: "success_last_page",
			Endpoint: &Endpoint{auth: okAuth(t), db: &DB{
				Follow: followsOf(t, []int64{3, 4}, nil), Puzzle: puzzlesOf(items, nil), User: usersOf(map[int64]string{3: "maker"}, nil),
			}},
			Input: &EndpointReq{limit: 2},
			WantResp: &EndpointResp{Items: []puzzles.Summary{
				puzzles.NewSummary(items[0], map[int64]string{3: maker}),
				puzzles.NewSummary(items[1], map[int64]string{3: maker}),
			}},
		},
	})

	t.Run("success_next_cursor", func(t *testing.T) {
		ep := &Endpoint{auth: okAuth(t), db: &DB{
			Follow: followsOf(t, []int64{3, 4}, nil), Puzzle: puzzlesOf(items, nil), User: usersOf(nil, nil),
		}}
		resp, err := ep.Execute(context.Background(), &EndpointReq{limit: 2, after: nil})
		if err != nil || len(resp.Items) != 2 || resp.NextCursor != "" {
			t.Fatalf("last page: %+v %v", resp, err)
		}
		// One more than the limit means there is a next page.
		ep.db.Puzzle = &mp.Puzzle{T: t, ListFunc: func(ctx context.Context, f puzzles.Filter) ([]*puzzles.Puzzle, error) {
			if f.Limit != 2 {
				t.Errorf("limit+1: %d", f.Limit)
			}
			return items, nil
		}}
		resp, err = ep.Execute(context.Background(), &EndpointReq{limit: 1})
		if err != nil || len(resp.Items) != 1 || resp.Items[0].ID != "p2" {
			t.Fatalf("page: %+v %v", resp, err)
		}
		next, ok := cursor.Decode[puzzles.Cursor](resp.NextCursor)
		if !ok || next.ID != "p2" || !next.CreatedAt.Equal(t0.Add(time.Hour)) {
			t.Fatalf("next cursor: %+v", next)
		}
	})
}
