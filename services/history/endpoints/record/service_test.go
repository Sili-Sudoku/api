package record

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	"github.com/Sili-Sudoku/api/services/history/model"
)

func ptr[T any](v T) *T { return &v }

func okAuth(t *testing.T) *mp.Auth {
	return &mp.Auth{T: t, GetUserIDFunc: func(ctx context.Context) (int64, error) { return 7, nil }}
}

func TestNew(t *testing.T) {
	auth, db := &mp.Auth{T: t}, &DB{}
	if ep := New(auth, db); ep.auth != auth || ep.db != db {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_BindPath(t *testing.T) {
	mp.RunBindPathTests(t, []mp.BindPathTest{
		{Name: "puzzleId", PathValues: map[string]string{"puzzleId": "p1"}, Input: &EndpointReq{}, Want: &EndpointReq{PuzzleID: "p1"}},
	})
}

func TestEndpointReq_Validate(t *testing.T) {
	mp.RunValidateTests(t, []mp.ValidateTest{
		{Name: "empty id", Input: &EndpointReq{Seconds: ptr(5)}, WantErr: true},
		{Name: "id with a space", Input: &EndpointReq{PuzzleID: "has space", Seconds: ptr(5)}, WantErr: true},
		{Name: "zero seconds", Input: &EndpointReq{PuzzleID: "ok", Seconds: ptr(0)}, WantErr: true},
		{Name: "too many seconds", Input: &EndpointReq{PuzzleID: "ok", Seconds: ptr(model.MaxSeconds + 1)}, WantErr: true},
		{Name: "solved in the future", Input: &EndpointReq{PuzzleID: "ok", SolvedAt: ptr(time.Now().Add(72 * time.Hour))}, WantErr: true},
		{Name: "time and date", Input: &EndpointReq{PuzzleID: "ok", Seconds: ptr(300), SolvedAt: ptr(time.Now())}},
		{Name: "neither time nor date", Input: &EndpointReq{PuzzleID: "ok"}},
	})
}

func TestEndpoint_Execute(t *testing.T) {
	now := time.Now()
	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name: "auth.GetUserID_error",
			Endpoint: &Endpoint{
				auth: &mp.Auth{T: t, GetUserIDFunc: func(ctx context.Context) (int64, error) { return 0, errors.New("authErr") }},
				db:   &DB{},
			},
			Input:   &EndpointReq{},
			WantErr: true,
		},
		{
			Name: "db.SolveHistory.Upsert_error",
			Endpoint: &Endpoint{
				auth: okAuth(t),
				db: &DB{SolveHistory: &mp.SolveHistory{T: t, UpsertFunc: func(ctx context.Context, userID int64, e model.Entry) (model.Entry, error) {
					return model.Entry{}, errors.New("dbErr")
				}}},
			},
			Input:   &EndpointReq{},
			WantErr: true,
		},
		{
			Name: "db.SolveHistory.Upsert_full",
			Endpoint: &Endpoint{
				auth: okAuth(t),
				db: &DB{SolveHistory: &mp.SolveHistory{T: t, UpsertFunc: func(ctx context.Context, userID int64, e model.Entry) (model.Entry, error) {
					return model.Entry{}, model.ErrFull
				}}},
			},
			Input:      &EndpointReq{},
			WantErr:    true,
			WantStatus: http.StatusConflict,
			WantCode:   "history_full",
		},
		{
			Name: "success",
			Endpoint: &Endpoint{
				auth: okAuth(t),
				db: &DB{SolveHistory: &mp.SolveHistory{T: t, UpsertFunc: func(ctx context.Context, userID int64, e model.Entry) (model.Entry, error) {
					if userID != 7 || e.PuzzleID != "p1" || *e.Seconds != 300 {
						t.Errorf("Upsert(%d, %+v)", userID, e)
					}
					return model.Entry{PuzzleID: "p1", Seconds: ptr(200), UpdatedAt: now}, nil
				}}},
			},
			Input:    &EndpointReq{PuzzleID: "p1", Seconds: ptr(300)},
			WantResp: &EndpointResp{PuzzleID: "p1", Seconds: ptr(200), UpdatedAt: now},
		},
	})
}
