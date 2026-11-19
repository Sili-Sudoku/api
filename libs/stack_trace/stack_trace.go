// Package stack_trace wraps an error with the names of the functions it travelled through.
package stack_trace

import (
	"reflect"
	"runtime"
	"strings"
)

type traced struct {
	caller, failed string
	err            error
}

func (t *traced) Error() string { return t.caller + " -> " + t.failed + ": " + t.err.Error() }
func (t *traced) Unwrap() error { return t.err }

// StackTrace names the calling function and the function value that failed. It returns nil for a nil error and keeps
// errors.Is and errors.As working.
func StackTrace(failed any, err error) error {
	if err == nil {
		return nil
	}
	caller := "unknown"
	if pc, _, _, ok := runtime.Caller(1); ok {
		caller = short(runtime.FuncForPC(pc).Name())
	}

	return &traced{caller: caller, failed: short(runtime.FuncForPC(reflect.ValueOf(failed).Pointer()).Name()), err: err}
}

// short turns "github.com/x/api/services/a/b.(*T).M-fm" into "b.(*T).M".
func short(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}

	return strings.TrimSuffix(name, "-fm")
}
