package integration

import (
	"encoding/json"
	"strings"
	"testing"

	historyroutes "github.com/Sili-Sudoku/api/services/history/routes"
)

func TestHistoryIsPrivateMergedAndFeedsSolveStats(t *testing.T) {
	e := newEnv(t)
	maker, ann, bob := e.signUp(t, "maker"), e.signUp(t, "ann"), e.signUp(t, "bob")
	id, _ := e.publish(t, "Nice one", "", asUser(maker)...)

	if w := do(e.h, "PUT", "/v1/me/history/"+id, `{"seconds":100}`); w.Code != 401 || !strings.Contains(w.Body.String(), `"code":"unauthenticated"`) {
		t.Fatalf("signed out put: %d %s", w.Code, w.Body)
	}
	if w := do(e.h, "GET", "/v1/me/history", ""); w.Code != 401 {
		t.Fatalf("signed out get: %d", w.Code)
	}
	if w := do(e.h, "DELETE", "/v1/me/history", ""); w.Code != 401 {
		t.Fatalf("signed out delete: %d", w.Code)
	}
	// Signed out comes first: bad input must not be looked at.
	if w := do(e.h, "PUT", "/v1/me/history/bad%20id", `nope`); w.Code != 401 {
		t.Fatalf("signed out, bad input: %d", w.Code)
	}
	if w := do(e.h, "GET", "/v1/me/history?limit=0", ""); w.Code != 401 {
		t.Fatalf("signed out, bad limit: %d", w.Code)
	}
	if w := do(e.h, "DELETE", "/v1/me/history", "", "X-Requested-With", ""); w.Code != 403 {
		t.Fatalf("csrf: %d", w.Code)
	}

	for _, c := range []struct{ body, code string }{
		{`{"seconds":0}`, "invalid"}, {`{"seconds":"x"}`, "bad_json"}, {`nope`, "bad_json"},
	} {
		if w := do(e.h, "PUT", "/v1/me/history/"+id, c.body, asUser(ann)...); w.Code != 400 || !strings.Contains(w.Body.String(), `"code":"`+c.code+`"`) {
			t.Fatalf("%s: %d %s", c.body, w.Code, w.Body)
		}
	}
	if w := do(e.h, "PUT", "/v1/me/history/bad%20id", `{"seconds":5}`, asUser(ann)...); w.Code != 400 {
		t.Fatalf("bad id: %d", w.Code)
	}
	do(e.h, "PUT", "/v1/me/history/"+id, `{"seconds":300,"solvedAt":"2026-11-01T10:00:00Z"}`, asUser(ann)...)
	w := do(e.h, "PUT", "/v1/me/history/"+id, `{"seconds":900}`, asUser(ann)...)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"seconds":300`) {
		t.Fatalf("a slower time must not replace: %d %s", w.Code, w.Body)
	}
	do(e.h, "PUT", "/v1/me/history/mini-sudoku", `{"seconds":40}`, asUser(ann)...)
	do(e.h, "PUT", "/v1/me/history/"+id, `{"seconds":100}`, asUser(bob)...)

	w = do(e.h, "GET", "/v1/me/history?limit=1", "", asUser(ann)...)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"puzzleId":"mini-sudoku"`) || !strings.Contains(w.Body.String(), `"title":null`) {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	var page struct{ NextCursor string }
	_ = json.Unmarshal(w.Body.Bytes(), &page)
	w = do(e.h, "GET", "/v1/me/history?limit=1&cursor="+page.NextCursor, "", asUser(ann)...)
	if !strings.Contains(w.Body.String(), `"title":"Nice one"`) || strings.Contains(w.Body.String(), "bob") {
		t.Fatalf("page 2: %s", w.Body)
	}
	if w := do(e.h, "GET", "/v1/me/history?cursor=zzz", "", asUser(ann)...); w.Code != 400 {
		t.Fatalf("bad cursor: %d", w.Code)
	}

	// Median of 300 and 100 (sorted: 100, 300; the upper middle is 300).
	if w := do(e.h, "GET", "/v1/puzzles/"+id+"/stats", ""); !strings.Contains(w.Body.String(), `"solves":2`) || !strings.Contains(w.Body.String(), `"medianSeconds":300`) {
		t.Fatalf("stats: %s", w.Body)
	}

	if w := do(e.h, "DELETE", "/v1/me/history", "", asUser(ann)...); w.Code != 204 {
		t.Fatalf("delete: %d", w.Code)
	}
	if w := do(e.h, "GET", "/v1/me/history", "", asUser(ann)...); !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("after delete: %s", w.Body)
	}
	if w := do(e.h, "GET", "/v1/me/history", "", asUser(bob)...); !strings.Contains(w.Body.String(), id) {
		t.Fatalf("bob lost his history: %s", w.Body)
	}
}

// The history program serves the history routes with the same session check, and nothing else.
func TestHistoryProgramServesOnlyHistory(t *testing.T) {
	e := newEnv(t)
	ann := e.signUp(t, "ann")
	h := historyroutes.New(e.history)

	if w := do(h, "GET", "/v1/me/history", ""); w.Code != 401 {
		t.Fatalf("history without a session: %d %s", w.Code, w.Body)
	}

	if w := do(h, "PUT", "/v1/me/history/mini-sudoku", `{"seconds":40}`, asUser(ann)...); w.Code != 200 {
		t.Fatalf("record with a session from the accounts program: %d %s", w.Code, w.Body)
	}

	if w := do(h, "GET", "/v1/me/history", "", asUser(ann)...); w.Code != 200 || !strings.Contains(w.Body.String(), `"puzzleId":"mini-sudoku"`) {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}

	if w := do(h, "GET", "/healthz", ""); w.Code != 204 {
		t.Fatalf("healthz: %d", w.Code)
	}

	for _, path := range []string{"/v1/me", "/v1/puzzles", "/v1/me/feed", "/v1/me/following"} {
		if w := do(h, "GET", path, "", asUser(ann)...); w.Code != 404 {
			t.Fatalf("%s must not be served here: %d", path, w.Code)
		}
	}
}
