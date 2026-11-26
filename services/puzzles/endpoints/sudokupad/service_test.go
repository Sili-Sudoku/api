package sudokupad

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
	mp "github.com/Sili-Sudoku/api/libs/mockprovider"
)

type fakeSource struct {
	data string
	err  error

	got []string
}

func (f *fakeSource) Fetch(ctx context.Context, id string) (string, error) {
	f.got = append(f.got, id)

	return f.data, f.err
}

func limiter(t *testing.T, allow bool, keys *[]string) *mp.Limiter {
	return &mp.Limiter{T: t, AllowFunc: func(key string) bool {
		*keys = append(*keys, key)
		return allow
	}}
}

func wantError(t *testing.T, err error, status int, code string) {
	t.Helper()

	var oe *http_outcome.ErrorResponse
	if !errors.As(err, &oe) || oe.Status != status || oe.Code != code {
		t.Fatalf("err = %v, want %d %s", err, status, code)
	}
}

func TestNew(t *testing.T) {
	ip, global, src := &mp.Limiter{T: t}, &mp.Limiter{T: t}, &fakeSource{}
	ep := New(ip, global, src, true)
	if ep.limitIP != ip || ep.limitGlobal != global || ep.source != src || !ep.trustProxy {
		t.Fatalf("New did not keep its dependencies: %+v", ep)
	}
}

func TestEndpointReq_Bind(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/v1/sudokupad?id=abc%2Fdef", nil)
	r.RemoteAddr = "10.0.0.1:5000"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")

	var req EndpointReq
	req.BindQuery(r)
	req.BindRequest(r)
	if req.ID != "abc/def" || req.remoteAddr != "10.0.0.1:5000" || req.forwardedFor != "1.2.3.4" {
		t.Fatalf("bound %+v", req)
	}
}

func TestExecute(t *testing.T) {
	for _, tt := range []struct {
		name              string
		id                string
		allowIP, allowAll bool
		srcData           string
		srcErr            error
		wantStatus        int
		wantCode          string
		wantKeys          string
		wantFetch         bool
	}{
		{name: "ok", id: "abc123", allowIP: true, allowAll: true, srcData: "sclXYZ", wantKeys: "ip:1.2.3.4 global", wantFetch: true},
		{name: "two-part id", id: "user/puzzle-1", allowIP: true, allowAll: true, srcData: "scl", wantKeys: "ip:1.2.3.4 global", wantFetch: true},
		{name: "empty id", id: "", wantStatus: 400, wantCode: "bad_id"},
		{name: "path tricks", id: "../admin", wantStatus: 400, wantCode: "bad_id"},
		{name: "query tricks", id: "abcd?x=1", wantStatus: 400, wantCode: "bad_id"},
		{name: "too long", id: strings.Repeat("a", 41), wantStatus: 400, wantCode: "bad_id"},
		{name: "per address limit", id: "abc123", allowIP: false, wantStatus: 429, wantCode: "rate_limited", wantKeys: "ip:1.2.3.4"},
		{name: "global limit", id: "abc123", allowIP: true, allowAll: false, wantStatus: 429, wantCode: "busy", wantKeys: "ip:1.2.3.4 global"},
		{name: "not found", id: "abc123", allowIP: true, allowAll: true, srcErr: ErrNotFound, wantStatus: 404, wantCode: "not_found", wantKeys: "ip:1.2.3.4 global", wantFetch: true},
		{name: "upstream down", id: "abc123", allowIP: true, allowAll: true, srcErr: errors.New("boom"), wantStatus: 502, wantCode: "upstream", wantKeys: "ip:1.2.3.4 global", wantFetch: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var keys []string
			src := &fakeSource{data: tt.srcData, err: tt.srcErr}
			ep := New(limiter(t, tt.allowIP, &keys), limiter(t, tt.allowAll, &keys), src, true)

			resp, err := ep.Execute(context.Background(), &EndpointReq{ID: tt.id, remoteAddr: "10.0.0.1:5000", forwardedFor: "1.2.3.4"})

			if got := strings.Join(keys, " "); got != tt.wantKeys {
				t.Errorf("limit keys = %q, want %q", got, tt.wantKeys)
			}
			if (len(src.got) > 0) != tt.wantFetch {
				t.Errorf("fetched %v, want fetch %v", src.got, tt.wantFetch)
			}
			if tt.wantStatus != 0 {
				wantError(t, err, tt.wantStatus, tt.wantCode)
				return
			}
			if err != nil || resp.Data != tt.srcData {
				t.Fatalf("resp = %+v, err = %v", resp, err)
			}
		})
	}
}

func TestHTTPSource(t *testing.T) {
	for _, tt := range []struct {
		name     string
		status   int
		body     string
		redirect bool
		want     string
		wantErr  error
		anyErr   bool
	}{
		{name: "puzzle", status: 200, body: "sclABC", want: "sclABC"},
		{name: "404", status: 404, wantErr: ErrNotFound},
		{name: "not scl", status: 200, body: "<html>", wantErr: ErrNotFound},
		{name: "server error", status: 500, anyErr: true},
		{name: "too big", status: 200, body: "scl" + strings.Repeat("a", maxDataBytes), anyErr: true},
		{name: "redirect not followed", redirect: true, anyErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var path, agent string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path, agent = r.URL.Path, r.UserAgent()
				if tt.redirect {
					http.Redirect(w, r, "http://example.invalid/", http.StatusFound)
					return
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			s := NewHTTPSource()
			s.BaseURL = srv.URL + "/api/puzzle/"
			got, err := s.Fetch(context.Background(), "user/abc")

			if path != "/api/puzzle/user/abc" || agent != "sili-sudoku-import" {
				t.Errorf("request path %q, agent %q", path, agent)
			}
			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			case tt.anyErr:
				if err == nil || errors.Is(err, ErrNotFound) {
					t.Fatalf("err = %v, want a failure that is not ErrNotFound", err)
				}
			default:
				if err != nil || got != tt.want {
					t.Fatalf("got %q, %v", got, err)
				}
			}
		})
	}
}
