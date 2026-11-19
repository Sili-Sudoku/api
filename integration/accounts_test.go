package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sili-Sudoku/api/libs/ratelimit"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	accountsroutes "github.com/Sili-Sudoku/api/services/accounts/routes"
	"github.com/Sili-Sudoku/api/services/accounts/session"
	puzzlesroutes "github.com/Sili-Sudoku/api/services/puzzles/routes"
)

func cookieOf(w *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == session.CookieName {
			return c
		}
	}
	return nil
}

func (e *env) signUp(t *testing.T, name string) string {
	t.Helper()
	w := do(e.h, "POST", "/v1/accounts", `{"username":"`+name+`","password":"a long password"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("sign up: %d %s", w.Code, w.Body)
	}
	return cookieOf(w).Value
}

func (e *env) logInToken(t *testing.T, name string) string {
	t.Helper()
	w := do(e.h, "POST", "/v1/sessions", `{"username":"`+name+`","password":"a long password"}`)
	if w.Code != 200 {
		t.Fatalf("log in: %d %s", w.Code, w.Body)
	}
	return cookieOf(w).Value
}

func asUser(token string) []string { return []string{"Cookie", session.CookieName + "=" + token} }

func TestSignUpSetsSafeCookieAndMeWorks(t *testing.T) {
	e := newEnv(t)
	w := do(e.h, "POST", "/v1/accounts", `{"username":"ann","password":"a long password","email":"a@b.cd"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	c := cookieOf(w)
	if c == nil || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie must be HttpOnly, Secure, SameSite=Lax: %+v", c)
	}
	if strings.Contains(w.Body.String(), "assword") {
		t.Fatalf("response leaks password data: %s", w.Body)
	}

	me := do(e.h, "GET", "/v1/me", "", asUser(c.Value)...)
	var out struct{ User accounts.UserJSON }
	_ = json.Unmarshal(me.Body.Bytes(), &out)
	if me.Code != 200 || out.User.Username != "ann" || out.User.Email != "a@b.cd" || out.User.IsAdmin {
		t.Fatalf("me: %d %s", me.Code, me.Body)
	}
	if code := do(e.h, "GET", "/v1/me", "").Code; code != http.StatusUnauthorized {
		t.Fatalf("me signed out: %d", code)
	}
	if code := do(e.h, "GET", "/v1/me", "", asUser("forged")...).Code; code != http.StatusUnauthorized {
		t.Fatalf("me with forged cookie: %d", code)
	}
}

func TestSignUpErrors(t *testing.T) {
	e := newEnv(t)
	e.signUp(t, "ann")
	for _, c := range []struct {
		body string
		code int
		err  string
	}{
		{`{"username":"ANN","password":"a long password"}`, 409, "username_taken"},
		{`{"username":"a","password":"a long password"}`, 400, "invalid_username"},
		{`{"username":"bob","password":"short"}`, 400, "weak_password"},
		{`{"username":"bob","password":"a long password","email":"x"}`, 400, "invalid_email"},
		{`not json`, 400, "bad_json"},
	} {
		w := do(e.h, "POST", "/v1/accounts", c.body)
		if w.Code != c.code || !strings.Contains(w.Body.String(), `"code":"`+c.err+`"`) {
			t.Errorf("%s: %d %s", c.body, w.Code, w.Body)
		}
	}
}

func TestLogInLogOut(t *testing.T) {
	e := newEnv(t)
	e.signUp(t, "ann")

	bad := do(e.h, "POST", "/v1/sessions", `{"username":"ann","password":"wrong password"}`)
	unknown := do(e.h, "POST", "/v1/sessions", `{"username":"nobody","password":"wrong password"}`)
	if bad.Code != 401 || bad.Body.String() != unknown.Body.String() {
		t.Fatalf("wrong password and unknown user must look the same: %s / %s", bad.Body, unknown.Body)
	}

	w := do(e.h, "POST", "/v1/sessions", `{"username":"Ann","password":"a long password"}`)
	if w.Code != 200 || cookieOf(w) == nil {
		t.Fatalf("log in: %d %s", w.Code, w.Body)
	}
	tok := cookieOf(w).Value
	out := do(e.h, "DELETE", "/v1/sessions/current", "", asUser(tok)...)
	if out.Code != 204 || cookieOf(out).MaxAge >= 0 {
		t.Fatalf("log out: %d %+v", out.Code, cookieOf(out))
	}
	if code := do(e.h, "GET", "/v1/me", "", asUser(tok)...).Code; code != 401 {
		t.Fatalf("session after log out: %d", code)
	}
}

func TestStateChangesNeedCSRFHeader(t *testing.T) {
	e := newEnv(t)
	for _, c := range [][2]string{
		{"POST", "/v1/accounts"}, {"POST", "/v1/sessions"}, {"DELETE", "/v1/sessions/current"},
		{"DELETE", "/v1/me"}, {"POST", "/v1/puzzles"}, {"POST", "/v1/puzzles/x/report"}, {"DELETE", "/v1/puzzles/x"},
	} {
		r := httptest.NewRequest(c[0], c[1], strings.NewReader(`{}`)) // no X-Requested-With
		w := httptest.NewRecorder()
		e.h.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), `"csrf"`) {
			t.Errorf("%s %s without the header: %d %s", c[0], c[1], w.Code, w.Body)
		}
	}
	r := httptest.NewRequest("POST", "/v1/accounts", strings.NewReader(`{}`))
	r.Header.Set("X-Requested-With", "other")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("wrong header value: %d", w.Code)
	}
}

func TestRateLimits(t *testing.T) {
	e := newEnv(t)
	e.accounts.Limits = accountsroutes.Limits{
		SignUp: ratelimit.New(2, time.Hour), LogInIP: ratelimit.New(100, time.Hour), LogInUser: ratelimit.New(2, time.Hour),
	}
	e.puzzles.Limits = puzzlesroutes.Limits{Publish: ratelimit.New(1, time.Hour)}
	e.build()

	e.signUp(t, "ann")
	e.signUp(t, "bob")
	if c := do(e.h, "POST", "/v1/accounts", `{"username":"cyril","password":"a long password"}`).Code; c != 429 {
		t.Fatalf("third sign-up from one address: %d", c)
	}

	do(e.h, "POST", "/v1/sessions", `{"username":"ann","password":"x"}`)
	do(e.h, "POST", "/v1/sessions", `{"username":"ann","password":"x"}`)
	if c := do(e.h, "POST", "/v1/sessions", `{"username":"ann","password":"a long password"}`).Code; c != 429 {
		t.Fatalf("third log-in for one account, even with the right password: %d", c)
	}

	if c := do(e.h, "POST", "/v1/puzzles", `{"a":1}`).Code; c != 202 {
		t.Fatalf("first publish: %d", c)
	}
	if c := do(e.h, "POST", "/v1/puzzles", `{"a":2}`).Code; c != 429 {
		t.Fatalf("second publish from one address: %d", c)
	}
}

func TestDeleteMeNeedsExplicitChoiceAndPassword(t *testing.T) {
	e := newEnv(t)
	tok := e.signUp(t, "ann")
	for _, body := range []string{`{"password":"a long password"}`, `{"password":"a long password","puzzles":"maybe"}`} {
		if c := do(e.h, "DELETE", "/v1/me", body, asUser(tok)...).Code; c != 400 {
			t.Errorf("%s: %d", body, c)
		}
	}
	if c := do(e.h, "DELETE", "/v1/me", `{"password":"nope nope nope","puzzles":"keep"}`, asUser(tok)...).Code; c != 401 {
		t.Fatalf("wrong password: %d", c)
	}
	if c := do(e.h, "DELETE", "/v1/me", `{"password":"a long password","puzzles":"keep"}`).Code; c != 401 {
		t.Fatalf("signed out: %d", c)
	}
}

func TestDeleteMeKillsSessionsAndHandlesPuzzles(t *testing.T) {
	for _, mode := range []string{"keep", "delete"} {
		t.Run(mode, func(t *testing.T) { deleteMe(t, mode) })
	}
}

// deleteMe is one subtest each, because every environment takes the committed database for the length of its test.
func deleteMe(t *testing.T, mode string) {
	e := newEnv(t)
	tok := e.signUp(t, "ann")
	other := do(e.h, "POST", "/v1/sessions", `{"username":"ann","password":"a long password"}`)
	tok2 := cookieOf(other).Value

	var pub map[string]string
	w := do(e.h, "POST", "/v1/puzzles", `{"title":"mine"}`, asUser(tok)...)
	_ = json.Unmarshal(w.Body.Bytes(), &pub)
	if pub["deleteToken"] != "" {
		t.Fatalf("a signed-in publisher owns the puzzle and gets no delete token: %v", pub)
	}

	del := do(e.h, "DELETE", "/v1/me", `{"password":"a long password","puzzles":"`+mode+`"}`, asUser(tok)...)
	if del.Code != 204 {
		t.Fatalf("%s: delete me: %d %s", mode, del.Code, del.Body)
	}
	for _, session := range []string{tok, tok2} {
		if c := do(e.h, "GET", "/v1/me", "", asUser(session)...).Code; c != 401 {
			t.Errorf("%s: a session of the deleted account still works: %d", mode, c)
		}
	}
	if c := do(e.h, "POST", "/v1/sessions", `{"username":"ann","password":"a long password"}`).Code; c != 401 {
		t.Errorf("%s: log in after delete: %d", mode, c)
	}
	got := do(e.h, "GET", "/v1/puzzles/"+pub["id"], "").Code
	if (mode == "keep" && got != 200) || (mode == "delete" && got != 404) {
		t.Errorf("%s: puzzle after account delete: %d", mode, got)
	}
}
