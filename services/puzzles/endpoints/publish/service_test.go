package publish

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

const puzzle = `{"metadata":{"title":"a","author":"Ann","tags":["classic","x"],"difficulty":"easy"},` +
	`"constraints":[{"use":"thermometer"},{"use":"killer"},{"use":"killer"}]}`

func signedOut(t *testing.T) *mp.OptionalAuth {
	return &mp.OptionalAuth{T: t, FindUserFunc: func(ctx context.Context) *accounts.User { return nil }}
}

func allow(t *testing.T, want string) *mp.Limiter {
	return &mp.Limiter{T: t, AllowFunc: func(key string) bool {
		if key != want {
			t.Errorf("limit key = %q, want %q", key, want)
		}
		return true
	}}
}

func creates(t *testing.T, created bool, err error) *DB {
	return &DB{Puzzle: &mp.Puzzle{T: t, CreateFunc: func(ctx context.Context, p *model.Puzzle) (bool, error) { return created, err }}}
}

func TestNew(t *testing.T) {
	auth, db, limit, validator := &mp.OptionalAuth{T: t}, &DB{}, &mp.Limiter{T: t}, &mp.Validator{T: t}
	ep := New(auth, db, limit, validator, true)
	if ep.auth != auth || ep.db != db || ep.limit != limit || ep.validator != validator || !ep.trustProxy {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_BindBody(t *testing.T) {
	big := `{"x":"` + strings.Repeat("a", model.MaxPuzzleBytes) + `"}`
	for _, tt := range []struct {
		name, body string
		wantBody   string
		wantStatus int
		wantCode   string
	}{
		{name: "too large", body: big, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "too_large"},
		{name: "not json", body: "nope", wantStatus: http.StatusBadRequest, wantCode: "bad_json"},
		{name: "kept as received", body: " " + puzzle + "\n", wantBody: " " + puzzle + "\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var req EndpointReq
			err := req.BindBody(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body)))
			if tt.wantStatus == 0 {
				if err != nil || string(req.Body) != tt.wantBody {
					t.Fatalf("body = %q, err = %v", req.Body, err)
				}
				return
			}
			var oe *http_outcome.ErrorResponse
			if !errors.As(err, &oe) || oe.Status != tt.wantStatus || oe.Code != tt.wantCode {
				t.Fatalf("err = %v, want %d %s", err, tt.wantStatus, tt.wantCode)
			}
		})
	}
}

func TestEndpointReq_BindRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "1.2.3.4:5"
	r.Header.Set("X-Forwarded-For", "9.9.9.9, 8.8.8.8")
	var req EndpointReq
	req.BindRequest(r)
	if req.remoteAddr != "1.2.3.4:5" || req.forwardedFor != "9.9.9.9, 8.8.8.8" {
		t.Fatalf("bound %+v", req)
	}
}

func TestEndpoint_Execute(t *testing.T) {
	validated := make(chan string, 1)
	validator := &mp.Validator{T: t, ValidateFunc: func(ctx context.Context, id string) error { validated <- id; return nil }}
	req := func(body string) *EndpointReq { return &EndpointReq{Body: []byte(body), remoteAddr: "1.2.3.4:5"} }

	mp.RunExecuteTests(t, []mp.ExecuteTest[EndpointReq, EndpointResp]{
		{
			Name:       "limit.Allow_refused",
			Endpoint:   &Endpoint{auth: signedOut(t), limit: &mp.Limiter{T: t, AllowFunc: func(string) bool { return false }}},
			Input:      req(puzzle),
			WantErr:    true,
			WantStatus: http.StatusTooManyRequests,
			WantCode:   "rate_limited",
		},
		{
			Name:       "describe_error",
			Endpoint:   &Endpoint{auth: signedOut(t), limit: allow(t, "ip:1.2.3.4")},
			Input:      req(`{"metadata":{"title":5}}`),
			WantErr:    true,
			WantStatus: http.StatusBadRequest,
			WantCode:   "invalid_puzzle",
		},
		{
			Name:     "db.Puzzle.Create_error",
			Endpoint: &Endpoint{auth: signedOut(t), limit: allow(t, "ip:1.2.3.4"), db: creates(t, false, errors.New("dbErr"))},
			Input:    req(puzzle),
			WantErr:  true,
		},
	})

	t.Run("success_signed_out_gets_a_token_and_validation_starts", func(t *testing.T) {
		var stored *model.Puzzle
		ep := &Endpoint{auth: signedOut(t), limit: allow(t, "ip:1.2.3.4"), validator: validator, db: &DB{Puzzle: &mp.Puzzle{T: t,
			CreateFunc: func(ctx context.Context, p *model.Puzzle) (bool, error) { stored = p; return true, nil }}}}
		resp, err := ep.Execute(context.Background(), req(puzzle))
		if err != nil || resp.DeleteToken == "" || resp.Status != "pending" || resp.StatusCode() != http.StatusAccepted {
			t.Fatalf("resp = %+v, err = %v", resp, err)
		}
		if stored.ID != resp.ID || !bytes.Equal(stored.Body, []byte(puzzle)) || stored.Status != model.Pending || stored.OwnerID != 0 {
			t.Fatalf("stored %+v", stored)
		}
		if stored.Title != "a" || stored.Author != "Ann" || stored.Difficulty != "easy" || len(stored.Tags) != 2 ||
			fmt.Sprint(stored.Variants) != "[killer thermometer]" {
			t.Fatalf("list fields: %+v", stored)
		}
		select {
		case id := <-validated:
			if id != resp.ID {
				t.Fatalf("validated %q", id)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("validation did not start")
		}
	})

	t.Run("success_repeat_has_no_token", func(t *testing.T) {
		ep := &Endpoint{auth: signedOut(t), limit: allow(t, "ip:1.2.3.4"), validator: validator, db: creates(t, false, nil)}
		resp, err := ep.Execute(context.Background(), req(puzzle))
		if err != nil || resp.DeleteToken != "" || resp.ID == "" {
			t.Fatalf("resp = %+v, err = %v", resp, err)
		}
		<-validated
	})

	t.Run("success_signed_in_owns_the_puzzle_and_gets_no_token", func(t *testing.T) {
		var stored *model.Puzzle
		ep := &Endpoint{
			auth:  &mp.OptionalAuth{T: t, FindUserFunc: func(ctx context.Context) *accounts.User { return &accounts.User{ID: 7, Username: "ann"} }},
			limit: allow(t, "user:ann"), validator: validator,
			db: &DB{Puzzle: &mp.Puzzle{T: t, CreateFunc: func(ctx context.Context, p *model.Puzzle) (bool, error) { stored = p; return true, nil }}},
		}
		resp, err := ep.Execute(context.Background(), req(puzzle))
		if err != nil || resp.DeleteToken != "" || stored.OwnerID != 7 {
			t.Fatalf("resp = %+v, err = %v, stored = %+v", resp, err, stored)
		}
		<-validated
	})

	t.Run("the_same_bytes_give_the_same_id", func(t *testing.T) {
		var ids []string
		for range 2 {
			ep := &Endpoint{auth: signedOut(t), limit: allow(t, "ip:1.2.3.4"), validator: validator, db: creates(t, true, nil)}
			resp, err := ep.Execute(context.Background(), req(puzzle))
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, resp.ID)
			<-validated
		}
		if ids[0] != ids[1] || len(ids[0]) != 16 {
			t.Fatalf("ids = %v", ids)
		}
	})

	t.Run("a_trusted_proxy_header_picks_the_limit_key", func(t *testing.T) {
		ep := &Endpoint{auth: signedOut(t), limit: allow(t, "ip:8.8.8.8"), trustProxy: true, validator: validator, db: creates(t, true, nil)}
		in := &EndpointReq{Body: []byte(puzzle), remoteAddr: "1.2.3.4:5", forwardedFor: "9.9.9.9, 8.8.8.8"}
		if _, err := ep.Execute(context.Background(), in); err != nil {
			t.Fatal(err)
		}
		<-validated
	})
}

func TestDescribe(t *testing.T) {
	long := strings.Repeat("a", 300)
	for name, body := range map[string]string{
		"title too long": fmt.Sprintf(`{"metadata":{"title":%q}}`, long),
		"title not text": `{"metadata":{"title":5}}`,
		"http source":    `{"metadata":{"title":"t","source":"http://example.com"}}`,
		"script source":  `{"metadata":{"title":"t","source":"javascript:alert(1)"}}`,
		"too many tags":  `{"metadata":{"title":"t","tags":["1","2","3","4","5","6","7","8","9","10","11","12","13","14","15","16","17","18","19","20","21","22","23","24","25","26","27","28","29","30","31"]}}`,
		"tag too long":   fmt.Sprintf(`{"metadata":{"title":"t","tags":[%q]}}`, strings.Repeat("a", 51)),
		"not an object":  `[1]`,
	} {
		if err := describe([]byte(body), &model.Puzzle{}); err == nil || !strings.HasPrefix(err.Error(), "invalid puzzle: ") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := describe([]byte(`{"metadata":{"title":"t","source":"https://example.com/x"}}`), &model.Puzzle{}); err != nil {
		t.Errorf("https source: %v", err)
	}
}
