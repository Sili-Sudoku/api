// Package mockprovider has the table runners and the mocks that the endpoint tests share.
package mockprovider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Sili-Sudoku/api/libs/http_outcome"
)

type Executor[Req any, Resp any] interface {
	Execute(ctx context.Context, req *Req) (*Resp, error)
}

type ExecuteTest[Req any, Resp any] struct {
	Name       string
	Endpoint   Executor[Req, Resp]
	Input      *Req
	WantErr    bool
	WantStatus int // when set, the error must be an *http_outcome.ErrorResponse with this status
	WantCode   string
	WantResp   *Resp
}

func RunExecuteTests[Req any, Resp any](t *testing.T, tests []ExecuteTest[Req, Resp]) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			resp, err := tt.Endpoint.Execute(context.Background(), tt.Input)
			if (err != nil) != tt.WantErr {
				t.Fatalf("err = %v, want error: %v", err, tt.WantErr)
			}
			if tt.WantStatus != 0 {
				var oe *http_outcome.ErrorResponse
				if !errors.As(err, &oe) || oe.Status != tt.WantStatus || (tt.WantCode != "" && oe.Code != tt.WantCode) {
					t.Fatalf("err = %v, want status %d code %q", err, tt.WantStatus, tt.WantCode)
				}
			}
			if tt.WantResp != nil && !reflect.DeepEqual(resp, tt.WantResp) {
				t.Fatalf("resp = %+v, want %+v", resp, tt.WantResp)
			}
		})
	}
}

type ValidateTest struct {
	Name    string
	Input   http_outcome.Validator
	WantErr bool
}

func RunValidateTests(t *testing.T, tests []ValidateTest) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if err := tt.Input.Validate(); (err != nil) != tt.WantErr {
				t.Fatalf("err = %v, want error: %v", err, tt.WantErr)
			}
		})
	}
}

type BindPathTest struct {
	Name       string
	PathValues map[string]string
	Input      http_outcome.PathBindable
	Want       any // compared with Input after binding
}

func RunBindPathTests(t *testing.T, tests []BindPathTest) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for k, v := range tt.PathValues {
				req.SetPathValue(k, v)
			}
			tt.Input.BindPath(req)
			if !reflect.DeepEqual(tt.Input, tt.Want) {
				t.Fatalf("bound %+v, want %+v", tt.Input, tt.Want)
			}
		})
	}
}
