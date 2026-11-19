package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type listed struct {
	Items []struct {
		ID, Title string
		Owner     *string
	}
	NextCursor string
}

func puzzleBody(title, extra string) string {
	return fmt.Sprintf(`{"metadata":{"title":%q,"tags":["classic"]},"constraints":[{"use":"killer"}]%s}`, title, extra)
}

// publish stores a puzzle and validates it at once, so the test does not wait for the background job.
func (e *env) publish(t *testing.T, title, extra string, hdr ...string) (id, token string) {
	t.Helper()
	w := do(e.h, "POST", "/v1/puzzles", puzzleBody(title, extra), hdr...)
	if w.Code != http.StatusAccepted {
		t.Fatalf("publish: %d %s", w.Code, w.Body)
	}
	var out map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if err := e.worker.Validate(context.Background(), out["id"]); err != nil {
		t.Fatal(err)
	}
	return out["id"], out["deleteToken"]
}

func (e *env) list(t *testing.T, query string) listed {
	t.Helper()
	w := do(e.h, "GET", "/v1/puzzles"+query, "")
	if w.Code != 200 {
		t.Fatalf("list %s: %d %s", query, w.Code, w.Body)
	}
	var out listed
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out
}

func TestListPagesFiltersAndOwnerName(t *testing.T) {
	e := newEnv(t)
	ann := e.signUp(t, "ann")
	for i := range 5 {
		hdr := []string(nil)
		if i%2 == 0 {
			hdr = asUser(ann)
		}
		e.publish(t, fmt.Sprintf("p%d", i), fmt.Sprintf(`,"n":%d`, i), hdr...)
	}

	seen := map[string]bool{}
	query := "?limit=2"
	for range 5 {
		page := e.list(t, query)
		for _, it := range page.Items {
			if seen[it.ID] {
				t.Fatalf("repeated %s", it.ID)
			}
			seen[it.ID] = true
		}
		if page.NextCursor == "" {
			break
		}
		query = "?limit=2&cursor=" + page.NextCursor
	}
	if len(seen) != 5 {
		t.Fatalf("saw %d of 5", len(seen))
	}

	byAnn := e.list(t, "?author=ANN")
	if len(byAnn.Items) != 3 || byAnn.Items[0].Owner == nil || *byAnn.Items[0].Owner != "ann" {
		t.Fatalf("by author: %+v", byAnn)
	}
	if n := len(e.list(t, "?author=nobody").Items); n != 0 {
		t.Fatalf("unknown author: %d", n)
	}
	if n := len(e.list(t, "?tag=classic&variant=killer").Items); n != 5 {
		t.Fatalf("tag+variant: %d", n)
	}
	if n := len(e.list(t, "?variant=arrow").Items); n != 0 {
		t.Fatalf("variant: %d", n)
	}
	for _, q := range []string{"?limit=0", "?limit=999", "?limit=x", "?cursor=@@", "?sort=hot"} {
		if c := do(e.h, "GET", "/v1/puzzles"+q, "").Code; c != 400 {
			t.Errorf("%s: %d", q, c)
		}
	}
}

func TestBodyIsHiddenUntilPublished(t *testing.T) {
	g := gate{release: make(chan struct{})}
	e := newEnvWith(t, g)
	ann := e.signUp(t, "ann")

	w := do(e.h, "POST", "/v1/puzzles", puzzleBody("pending", ""), asUser(ann)...)
	var pub map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &pub)
	for _, hdr := range [][]string{nil, asUser(ann)} {
		body := do(e.h, "GET", "/v1/puzzles/"+pub["id"], "", hdr...).Body.String()
		if strings.Contains(body, `"puzzle"`) || !strings.Contains(body, `"status":"pending"`) {
			t.Fatalf("pending: %s", body)
		}
	}
	close(g.release)
}

func TestRejectedBodyIsForTheOwnerOnly(t *testing.T) {
	e := newEnvWith(t, reject{})
	ann := e.signUp(t, "ann")
	e.signUp(t, "bob")
	id, _ := e.publish(t, "bad", "", asUser(ann)...)

	owner := do(e.h, "GET", "/v1/puzzles/"+id, "", asUser(ann)...).Body.String()
	if !strings.Contains(owner, `"puzzle"`) || !strings.Contains(owner, "no solution") || !strings.Contains(owner, `"mine":true`) {
		t.Fatalf("owner view: %s", owner)
	}
	for _, hdr := range [][]string{nil, asUser(e.logInToken(t, "bob"))} {
		if body := do(e.h, "GET", "/v1/puzzles/"+id, "", hdr...).Body.String(); strings.Contains(body, `"puzzle"`) {
			t.Fatalf("rejected body leaked: %s", body)
		}
	}
	if n := len(e.list(t, "").Items); n != 0 {
		t.Fatalf("a rejected puzzle was listed: %d", n)
	}
}

func TestManySolutionsIsAWarningNotABlock(t *testing.T) {
	e := newEnvWith(t, many{})
	id, _ := e.publish(t, "open", "")
	body := do(e.h, "GET", "/v1/puzzles/"+id, "").Body.String()
	if !strings.Contains(body, `"puzzle"`) || !strings.Contains(body, `"many_solutions"`) {
		t.Fatalf("%s", body)
	}
}

func TestReportNeedsSignInAndThreeHide(t *testing.T) {
	e := newEnv(t)
	id, _ := e.publish(t, "p", "")
	if c := do(e.h, "POST", "/v1/puzzles/"+id+"/report", "").Code; c != 401 {
		t.Fatalf("signed out report: %d", c)
	}
	var tokens []string
	for _, n := range []string{"ann", "bob", "cyd"} {
		tokens = append(tokens, e.signUp(t, n))
	}
	for i, tok := range tokens {
		for range 2 { // a second report by one user changes nothing
			if c := do(e.h, "POST", "/v1/puzzles/"+id+"/report", "", asUser(tok)...).Code; c != 204 {
				t.Fatalf("report: %d", c)
			}
		}
		want := 1
		if i == 2 {
			want = 0
		}
		if n := len(e.list(t, "").Items); n != want {
			t.Fatalf("after %d reporters the list has %d items, want %d", i+1, n, want)
		}
	}
	if c := do(e.h, "GET", "/v1/puzzles/"+id, "").Code; c != 200 {
		t.Fatalf("a reported puzzle still opens by link: %d", c)
	}
	if c := do(e.h, "POST", "/v1/puzzles/missing/report", "", asUser(tokens[0])...).Code; c != 404 {
		t.Fatalf("report missing: %d", c)
	}
}

func TestClaimThenOwnerDeletes(t *testing.T) {
	e := newEnv(t)
	ann := e.signUp(t, "ann")
	bob := e.signUp(t, "bob")
	id, token := e.publish(t, "mine", "")
	if token == "" {
		t.Fatal("signed-out publish must return a token")
	}

	claim := func(tok string, hdr ...string) int {
		return do(e.h, "PUT", "/v1/puzzles/"+id+"/owner", "", append(hdr, "X-Delete-Token", tok)...).Code
	}
	if c := claim(token); c != 401 {
		t.Fatalf("claim signed out: %d", c)
	}
	if c := claim("wrong", asUser(ann)...); c != 403 {
		t.Fatalf("claim with a wrong token: %d", c)
	}
	if c := claim(token, asUser(ann)...); c != 204 {
		t.Fatalf("claim: %d", c)
	}
	if c := claim(token, asUser(ann)...); c != 204 {
		t.Fatalf("claim again by the owner: %d", c)
	}
	if c := claim(token, asUser(bob)...); c != 409 {
		t.Fatalf("claim of an owned puzzle: %d", c)
	}
	if it := e.list(t, "").Items; len(it) != 1 || it[0].Owner == nil || *it[0].Owner != "ann" {
		t.Fatalf("owner shown in the list: %+v", it)
	}
	if c := do(e.h, "DELETE", "/v1/puzzles/"+id, "", asUser(bob)...).Code; c != 403 {
		t.Fatalf("another user deleting: %d", c)
	}
	if c := do(e.h, "DELETE", "/v1/puzzles/"+id, "", asUser(ann)...).Code; c != 204 {
		t.Fatalf("owner deleting: %d", c)
	}
	if c := do(e.h, "GET", "/v1/puzzles/"+id, "").Code; c != 404 {
		t.Fatalf("after delete: %d", c)
	}
}

func TestPublishRejectsBadTextFields(t *testing.T) {
	e := newEnv(t)
	for _, b := range []string{
		`{"metadata":{"title":"t","source":"javascript:alert(1)"}}`,
		fmt.Sprintf(`{"metadata":{"title":%q}}`, strings.Repeat("x", 201)),
	} {
		w := do(e.h, "POST", "/v1/puzzles", b)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_puzzle") {
			t.Errorf("%s: %d %s", b, w.Code, w.Body)
		}
	}
}
