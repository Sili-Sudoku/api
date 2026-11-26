package boot

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIPFromHeader(t *testing.T) {
	cases := []struct {
		name      string
		client    string
		forwarded string
		want      string
	}{
		{"gateway header replaces the chain", "203.0.113.7", "198.51.100.1, 10.0.0.1", "203.0.113.7"},
		{"header with spaces", "  203.0.113.7 ", "", "203.0.113.7"},
		{"no header drops a forged chain", "", "6.6.6.6", ""},
		{"no header, no chain", "", "", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got string
			h := clientIPFromHeader("X-Client-IP", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("X-Forwarded-For")
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if c.client != "" {
				req.Header.Set("X-Client-IP", c.client)
			}
			if c.forwarded != "" {
				req.Header.Set("X-Forwarded-For", c.forwarded)
			}

			h.ServeHTTP(httptest.NewRecorder(), req)

			if got != c.want {
				t.Errorf("X-Forwarded-For = %q, want %q", got, c.want)
			}
		})
	}
}

func TestCheckValidator(t *testing.T) {
	if err := checkValidator("", true); err == nil {
		t.Error("an empty VALIDATE_CMD must be refused when the validator is required")
	}

	for _, c := range []struct {
		cmd      string
		required bool
	}{{"/bin/validate", true}, {"/bin/validate", false}, {"", false}} {
		if err := checkValidator(c.cmd, c.required); err != nil {
			t.Errorf("checkValidator(%q, %v) = %v, want nil", c.cmd, c.required, err)
		}
	}
}
