package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sili-Sudoku/api/libs/validate"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
	"github.com/Sili-Sudoku/api/services/puzzles/worker"
)

type reject struct{}

func (reject) Validate(context.Context, []byte) (validate.Verdict, error) {
	return validate.Verdict{Valid: false, Solutions: "none", Errors: []string{"no solution"}}, nil
}

// many accepts a puzzle that has more than one solution.
type many struct{}

func (many) Validate(context.Context, []byte) (validate.Verdict, error) {
	return validate.Verdict{Valid: true, Solutions: "many"}, nil
}

// gate holds validation until the test releases it, so "pending" can be observed.
type gate struct{ release chan struct{} }

func (g gate) Validate(context.Context, []byte) (validate.Verdict, error) {
	<-g.release
	return validate.Verdict{Valid: true, Solutions: "one"}, nil
}

func testServer(t *testing.T, v validate.Validator) (http.Handler, *worker.Worker) {
	e := newEnvWith(t, v)
	return e.h, e.worker
}

func do(h http.Handler, method, path, body string, hdr ...string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("X-Requested-With", "sudoku")
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestPublishValidateGetDelete(t *testing.T) {
	g := gate{release: make(chan struct{})}
	h, svc := testServer(t, g)

	w := do(h, "POST", "/v1/puzzles", `{"title":"t"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("publish: %d %s", w.Code, w.Body)
	}
	var pub map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &pub)
	id, token := pub["id"], pub["deleteToken"]
	if id == "" || token == "" {
		t.Fatalf("missing id/token: %v", pub)
	}

	// Pending puzzles do not expose their body.
	if body := do(h, "GET", "/v1/puzzles/"+id, "").Body.String(); strings.Contains(body, `"puzzle"`) {
		t.Fatalf("pending puzzle leaked body: %s", body)
	}
	close(g.release) // lets the background job and our own call finish
	if err := svc.Validate(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if body := do(h, "GET", "/v1/puzzles/"+id, "").Body.String(); !strings.Contains(body, `"puzzle"`) {
		t.Fatalf("published puzzle has no body: %s", body)
	}

	// Same bytes, same id, no second token.
	again := do(h, "POST", "/v1/puzzles", `{"title":"t"}`)
	if strings.Contains(again.Body.String(), "deleteToken") {
		t.Fatal("repeat publish returned a token")
	}

	if c := do(h, "DELETE", "/v1/puzzles/"+id, "", "X-Delete-Token", "nope").Code; c != http.StatusForbidden {
		t.Fatalf("wrong token: %d", c)
	}
	if c := do(h, "DELETE", "/v1/puzzles/"+id, "", "X-Delete-Token", token).Code; c != http.StatusNoContent {
		t.Fatalf("delete: %d", c)
	}
	if c := do(h, "GET", "/v1/puzzles/"+id, "").Code; c != http.StatusNotFound {
		t.Fatalf("after delete: %d", c)
	}
}

func TestRejectedPuzzleHidesBody(t *testing.T) {
	h, svc := testServer(t, reject{})
	var pub map[string]string
	_ = json.Unmarshal(do(h, "POST", "/v1/puzzles", `{"a":1}`).Body.Bytes(), &pub)
	_ = svc.Validate(context.Background(), pub["id"])
	body := do(h, "GET", "/v1/puzzles/"+pub["id"], "").Body.String()
	if strings.Contains(body, `"puzzle"`) || !strings.Contains(body, "rejected") {
		t.Fatalf("unexpected: %s", body)
	}
}

func TestRejectsBadInput(t *testing.T) {
	h, _ := testServer(t, validate.Accept{})
	if c := do(h, "POST", "/v1/puzzles", "not json").Code; c != http.StatusBadRequest {
		t.Fatalf("non-JSON: %d", c)
	}
	if c := do(h, "POST", "/v1/puzzles", strings.Repeat("a", puzzles.MaxPuzzleBytes+1)).Code; c != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize: %d", c)
	}
}
