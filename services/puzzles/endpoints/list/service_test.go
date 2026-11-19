package list

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sili-Sudoku/api/libs/cursor"
	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

func TestNew(t *testing.T) {
	db := &DB{}
	if ep := New(db); ep.db != db {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_BindQuery(t *testing.T) {
	var req EndpointReq
	req.BindQuery(httptest.NewRequest(http.MethodGet, "/?tag=t&variant=v&difficulty=d&author=ann&limit=5&cursor=c&sort=top", nil))
	f := req.filter
	if req.rawAuthor != "ann" || req.rawCursor != "c" || req.rawLimit != "5" || req.rawSort != "top" ||
		f.Tag != "t" || f.Variant != "v" || f.Difficulty != "d" {
		t.Fatalf("bound %+v", req)
	}
}

func TestEndpointReq_Validate(t *testing.T) {
	t0 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	newest := cursor.Encode(model.Cursor{CreatedAt: t0, ID: "p1"})
	top := cursor.Encode(model.Cursor{Offset: 20})
	mp.RunValidateTests(t, []mp.ValidateTest{
		{Name: "limit is not a number", Input: &EndpointReq{rawLimit: "x"}, WantErr: true},
		{Name: "limit is zero", Input: &EndpointReq{rawLimit: "0"}, WantErr: true},
		{Name: "limit is too big", Input: &EndpointReq{rawLimit: "51"}, WantErr: true},
		{Name: "sort is unknown", Input: &EndpointReq{rawSort: "oldest"}, WantErr: true},
		{Name: "cursor is not a cursor", Input: &EndpointReq{rawCursor: "zzz"}, WantErr: true},
		{Name: "cursor without a position", Input: &EndpointReq{rawCursor: cursor.Encode(model.Cursor{})}, WantErr: true},
		{Name: "cursor with a negative offset", Input: &EndpointReq{rawCursor: cursor.Encode(model.Cursor{Offset: -1})}, WantErr: true},
		{Name: "defaults", Input: &EndpointReq{}},
		{Name: "newest with a cursor", Input: &EndpointReq{rawLimit: "50", rawSort: "newest", rawCursor: newest}},
		{Name: "top with a cursor", Input: &EndpointReq{rawSort: "top", rawCursor: top}},
	})

	t.Run("fills the filter", func(t *testing.T) {
		req := &EndpointReq{rawLimit: "7", rawSort: "top", rawCursor: top}
		if err := req.Validate(); err != nil || req.filter.Limit != 7 || !req.filter.Top || req.filter.Offset != 20 || req.filter.After != nil {
			t.Fatalf("top: %+v %v", req.filter, err)
		}
		req = &EndpointReq{rawCursor: newest}
		if err := req.Validate(); err != nil || req.filter.Limit != defaultLimit || req.filter.Top || req.filter.Offset != 0 ||
			req.filter.After == nil || req.filter.After.ID != "p1" {
			t.Fatalf("newest: %+v %v", req.filter, err)
		}
	})
}

func TestEndpoint_Execute(t *testing.T) {
	t0 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	ann := "ann"
	puzzles := []*model.Puzzle{
		{ID: "p3", OwnerID: 7, CreatedAt: t0.Add(2 * time.Hour)},
		{ID: "p2", CreatedAt: t0.Add(time.Hour)},
		{ID: "p1", OwnerID: 7, CreatedAt: t0},
	}
	listOf := func(t *testing.T, items []*model.Puzzle, err error, got *model.Filter) *mp.Puzzle {
		return &mp.Puzzle{T: t, ListFunc: func(ctx context.Context, f model.Filter) ([]*model.Puzzle, error) {
			if got != nil {
				*got = f
			}
			return items, err
		}}
	}
	usernames := &mp.User{T: t, UsernamesFunc: func(ctx context.Context, ids []int64) (map[int64]string, error) { return map[int64]string{7: ann}, nil }}
	owner := func(id int64) *accounts.User { return &accounts.User{ID: id} }

	var got model.Filter
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name: "db.User.FindByName_error",
			Endpoint: &Endpoint{db: &DB{User: &mp.User{T: t, FindByNameFunc: func(ctx context.Context, name string) (*accounts.User, error) {
				return nil, errors.New("dbErr")
			}}}},
			Input:   &EndpointReq{rawAuthor: "ann"},
			WantErr: true,
		},
		{
			Name: "db.User.FindByName_unknown_author_lists_nothing",
			Endpoint: &Endpoint{db: &DB{User: &mp.User{T: t, FindByNameFunc: func(ctx context.Context, name string) (*accounts.User, error) {
				return nil, nil
			}}}},
			Input:    &EndpointReq{rawAuthor: "nobody"},
			WantResp: &EndpointResp{Items: []model.Summary{}},
		},
		{
			Name:     "db.Puzzle.List_error",
			Endpoint: &Endpoint{db: &DB{Puzzle: listOf(t, nil, errors.New("dbErr"), nil)}},
			Input:    &EndpointReq{filter: model.Filter{Limit: 2}},
			WantErr:  true,
		},
		{
			Name: "success_by_author_last_page",
			Endpoint: &Endpoint{db: &DB{Puzzle: listOf(t, puzzles[:1], nil, &got), User: &mp.User{T: t,
				FindByNameFunc: func(ctx context.Context, name string) (*accounts.User, error) { return owner(7), nil },
				UsernamesFunc:  usernames.UsernamesFunc}}},
			Input: &EndpointReq{rawAuthor: "ann", filter: model.Filter{Limit: 2, Tag: "x"}},
			WantResp: &EndpointResp{Items: []model.Summary{func() model.Summary {
				return model.NewSummary(puzzles[0], map[int64]string{7: ann})
			}()}},
		},
		{
			Name:     "success_empty",
			Endpoint: &Endpoint{db: &DB{Puzzle: listOf(t, nil, nil, nil), User: usernames}},
			Input:    &EndpointReq{filter: model.Filter{Limit: 2}},
			WantResp: &EndpointResp{Items: []model.Summary{}},
		},
	})
	if len(got.OwnerIDs) != 1 || got.OwnerIDs[0] != 7 || got.Tag != "x" || got.Limit != 3 {
		t.Fatalf("filter sent to the db: %+v", got)
	}

	t.Run("success_newest_next_cursor", func(t *testing.T) {
		ep := &Endpoint{db: &DB{Puzzle: listOf(t, puzzles, nil, nil), User: usernames}}
		resp, err := ep.Execute(context.Background(), &EndpointReq{filter: model.Filter{Limit: 2}})
		if err != nil || len(resp.Items) != 2 || resp.Items[0].ID != "p3" || *resp.Items[0].Owner != ann || resp.Items[1].Owner != nil {
			t.Fatalf("page: %+v %v", resp, err)
		}
		next, ok := cursor.Decode[model.Cursor](resp.NextCursor)
		if !ok || next.ID != "p2" || !next.CreatedAt.Equal(t0.Add(time.Hour)) || next.Offset != 0 {
			t.Fatalf("next cursor: %+v", next)
		}
	})

	t.Run("success_top_pages_by_offset", func(t *testing.T) {
		var sent model.Filter
		ep := &Endpoint{db: &DB{Puzzle: listOf(t, puzzles, nil, &sent), User: usernames}}
		req := &EndpointReq{rawSort: "top", rawCursor: cursor.Encode(model.Cursor{Offset: 4}), rawLimit: "2"}
		if err := req.Validate(); err != nil {
			t.Fatal(err)
		}
		resp, err := ep.Execute(context.Background(), req)
		if err != nil || len(resp.Items) != 2 {
			t.Fatalf("page: %+v %v", resp, err)
		}
		if !sent.Top || sent.Offset != 4 || sent.After != nil || sent.Limit != 3 {
			t.Fatalf("filter sent to the db: %+v", sent)
		}
		next, ok := cursor.Decode[model.Cursor](resp.NextCursor)
		if !ok || next.Offset != 6 || next.ID != "" {
			t.Fatalf("next cursor: %+v", next)
		}
	})

	t.Run("db.User.Usernames_error_still_serves_the_page", func(t *testing.T) {
		ep := &Endpoint{db: &DB{Puzzle: listOf(t, puzzles[:1], nil, nil), User: &mp.User{T: t,
			UsernamesFunc: func(ctx context.Context, ids []int64) (map[int64]string, error) { return nil, errors.New("dbErr") }}}}
		resp, err := ep.Execute(context.Background(), &EndpointReq{filter: model.Filter{Limit: 2}})
		if err != nil || len(resp.Items) != 1 || resp.Items[0].Owner != nil {
			t.Fatalf("page: %+v %v", resp, err)
		}
	})
}
