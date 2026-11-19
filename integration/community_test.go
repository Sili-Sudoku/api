package integration

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestRatingsNeedSignInAndAreSummarized(t *testing.T) {
	e := newEnv(t)
	maker, ann := e.signUp(t, "maker"), e.signUp(t, "ann")
	id, _ := e.publish(t, "p", "", asUser(maker)...)
	url := "/v1/puzzles/" + id + "/ratings"

	if w := do(e.h, "PUT", url, `{"stars":5}`); w.Code != 401 {
		t.Fatalf("signed out: %d", w.Code)
	}
	if w := do(e.h, "PUT", url, `{"stars":5}`, asUser(maker)...); w.Code != 403 || !strings.Contains(w.Body.String(), "own_puzzle") {
		t.Fatalf("own puzzle: %d %s", w.Code, w.Body)
	}
	if w := do(e.h, "PUT", url, `{"stars":9}`, asUser(ann)...); w.Code != 400 {
		t.Fatalf("bad stars: %d", w.Code)
	}
	w := do(e.h, "PUT", url, `{"stars":4,"difficulty":2}`, asUser(ann)...)
	if w.Code != 200 {
		t.Fatalf("rate: %d %s", w.Code, w.Body)
	}
	var got struct {
		RatingCount    int
		Rating         float64
		FeltDifficulty float64
		Mine           *struct{ Stars, Difficulty int }
	}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.RatingCount != 1 || got.Rating != 4 || got.FeltDifficulty != 2 || got.Mine == nil || got.Mine.Stars != 4 {
		t.Fatalf("response: %s", w.Body)
	}
	// The list shows the summary, and sort=top works.
	if w := do(e.h, "GET", "/v1/puzzles?sort=top", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"rating":4`) {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	if w := do(e.h, "GET", "/v1/puzzles?sort=bogus", ""); w.Code != 400 {
		t.Fatalf("bad sort: %d", w.Code)
	}
	// A signed-out reader sees the totals but no "mine".
	w = do(e.h, "GET", url, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"mine":null`) {
		t.Fatalf("read: %d %s", w.Code, w.Body)
	}
}

func TestCommentsPlainTextReportsAndDelete(t *testing.T) {
	e := newEnv(t)
	maker, ann, bob, cy := e.signUp(t, "maker"), e.signUp(t, "ann"), e.signUp(t, "bob"), e.signUp(t, "cyd")
	id, _ := e.publish(t, "p", "", asUser(maker)...)
	url := "/v1/puzzles/" + id + "/comments"

	if w := do(e.h, "POST", url, `{"text":"hi"}`); w.Code != 401 {
		t.Fatalf("signed out: %d", w.Code)
	}
	if w := do(e.h, "POST", url, `{"text":"  "}`, asUser(ann)...); w.Code != 400 {
		t.Fatalf("empty: %d", w.Code)
	}
	w := do(e.h, "POST", url, `{"text":"<script>alert(1)</script>"}`, asUser(ann)...)
	if w.Code != 201 {
		t.Fatalf("post: %d %s", w.Code, w.Body)
	}
	var c struct {
		ID     int64
		Author string
		Text   string
		Mine   bool
	}
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	if c.Text != "<script>alert(1)</script>" || c.Author != "ann" || !c.Mine {
		t.Fatalf("comment: %+v", c)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type %q", ct)
	}
	path := func(suffix string) string { return "/v1/comments/" + itoa(c.ID) + suffix }

	if w := do(e.h, "GET", url+"?limit=0", ""); w.Code != 400 {
		t.Fatalf("limit: %d", w.Code)
	}
	if w := do(e.h, "GET", url+"?cursor=zzz", ""); w.Code != 400 {
		t.Fatalf("cursor: %d", w.Code)
	}
	// Report: signed in only; three reports hide it from the list.
	if w := do(e.h, "POST", path("/report"), ""); w.Code != 401 {
		t.Fatalf("report signed out: %d", w.Code)
	}
	for _, tok := range []string{bob, cy, maker} {
		if w := do(e.h, "POST", path("/report"), "", asUser(tok)...); w.Code != http.StatusNoContent {
			t.Fatalf("report: %d %s", w.Code, w.Body)
		}
	}
	if w := do(e.h, "GET", url, ""); !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("still listed: %s", w.Body)
	}
	// Delete: not by others, yes by the author.
	if w := do(e.h, "DELETE", path(""), "", asUser(bob)...); w.Code != 403 {
		t.Fatalf("delete other: %d", w.Code)
	}
	if w := do(e.h, "DELETE", path(""), "", asUser(ann)...); w.Code != 204 {
		t.Fatalf("delete own: %d", w.Code)
	}
	if w := do(e.h, "DELETE", "/v1/comments/abc", "", asUser(ann)...); w.Code != 404 {
		t.Fatalf("bad id: %d", w.Code)
	}
	if w := do(e.h, "GET", "/v1/puzzles/"+id+"/stats", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"medianSeconds":null`) {
		t.Fatalf("stats: %d %s", w.Code, w.Body)
	}
}

func TestCommentRateLimit(t *testing.T) {
	e := newEnv(t)
	maker, ann := e.signUp(t, "maker"), e.signUp(t, "ann")
	id, _ := e.publish(t, "p", "", asUser(maker)...)
	var last int
	for range 31 {
		last = do(e.h, "POST", "/v1/puzzles/"+id+"/comments", `{"text":"x"}`, asUser(ann)...).Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("31st comment: %d", last)
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestProfilesFollowAndFeed(t *testing.T) {
	e := newEnv(t)
	maker, ann, bob := e.signUp(t, "maker"), e.signUp(t, "ann"), e.signUp(t, "bob")
	e.publish(t, "by maker", "", asUser(maker)...)
	e.publish(t, "by bob", `,"n":1`, asUser(bob)...)
	e.publish(t, "anonymous", `,"n":2`)

	// Bio: plain text, 500 characters at most, yours only.
	if w := do(e.h, "PATCH", "/v1/me", `{"bio":"hi"}`); w.Code != 401 {
		t.Fatalf("signed out: %d", w.Code)
	}
	if w := do(e.h, "PATCH", "/v1/me", `{"bio":"`+strings.Repeat("x", 501)+`"}`, asUser(maker)...); w.Code != 400 {
		t.Fatalf("long bio: %d", w.Code)
	}
	if w := do(e.h, "PATCH", "/v1/me", `{"bio":"  I set <b>puzzles</b>  "}`, asUser(maker)...); w.Code != 200 || !strings.Contains(w.Body.String(), `"bio":"I set \u003cb\u003epuzzles\u003c/b\u003e"`) {
		t.Fatalf("set bio: %d %s", w.Code, w.Body)
	}

	if w := do(e.h, "GET", "/v1/users/nobody", ""); w.Code != 404 {
		t.Fatalf("missing user: %d", w.Code)
	}
	if w := do(e.h, "GET", "/v1/users/MAKER", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"followers":0`) || strings.Contains(w.Body.String(), "email") {
		t.Fatalf("profile: %d %s", w.Code, w.Body)
	}

	// Follow: signed in, not yourself, idempotent, visible in the profile.
	if w := do(e.h, "POST", "/v1/users/maker/follow", ""); w.Code != 401 {
		t.Fatalf("follow signed out: %d", w.Code)
	}
	if w := do(e.h, "POST", "/v1/users/ann/follow", "", asUser(ann)...); w.Code != 400 {
		t.Fatalf("follow self: %d", w.Code)
	}
	if w := do(e.h, "POST", "/v1/users/nobody/follow", "", asUser(ann)...); w.Code != 404 {
		t.Fatalf("follow missing: %d", w.Code)
	}
	for range 2 {
		if w := do(e.h, "POST", "/v1/users/maker/follow", "", asUser(ann)...); w.Code != 204 {
			t.Fatalf("follow: %d %s", w.Code, w.Body)
		}
	}
	w := do(e.h, "GET", "/v1/users/maker", "", asUser(ann)...)
	if !strings.Contains(w.Body.String(), `"followers":1`) || !strings.Contains(w.Body.String(), `"followed":true`) || !strings.Contains(w.Body.String(), `"me":false`) {
		t.Fatalf("profile as follower: %s", w.Body)
	}
	if w := do(e.h, "GET", "/v1/users/maker", ""); !strings.Contains(w.Body.String(), `"followed":false`) {
		t.Fatalf("profile signed out: %s", w.Body)
	}

	// The feed holds only the puzzles of followed creators.
	w = do(e.h, "GET", "/v1/me/feed", "", asUser(ann)...)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "by maker") || strings.Contains(w.Body.String(), "by bob") || strings.Contains(w.Body.String(), "anonymous") {
		t.Fatalf("feed: %d %s", w.Code, w.Body)
	}
	if w := do(e.h, "GET", "/v1/me/feed", "", asUser(bob)...); !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("empty feed: %s", w.Body)
	}
	if w := do(e.h, "GET", "/v1/me/feed", ""); w.Code != 401 {
		t.Fatalf("feed signed out: %d", w.Code)
	}
	if w := do(e.h, "GET", "/v1/me/following", "", asUser(ann)...); !strings.Contains(w.Body.String(), `"username":"maker"`) {
		t.Fatalf("following: %s", w.Body)
	}

	// Author filter on the list works with the creator's name.
	if w := do(e.h, "GET", "/v1/puzzles?author=bob", ""); !strings.Contains(w.Body.String(), "by bob") || strings.Contains(w.Body.String(), "by maker") {
		t.Fatalf("author list: %s", w.Body)
	}

	if w := do(e.h, "DELETE", "/v1/users/maker/follow", "", asUser(ann)...); w.Code != 204 {
		t.Fatalf("unfollow: %d", w.Code)
	}
	if w := do(e.h, "GET", "/v1/me/feed", "", asUser(ann)...); !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("feed after unfollow: %s", w.Body)
	}
}
