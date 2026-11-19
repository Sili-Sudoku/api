// Package http_outcome turns a plain function of a request into an http.HandlerFunc, with the API's {error, code} bodies.
package http_outcome

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

type Endpoint[Req any, Resp any] func(ctx context.Context, req *Req) (*Resp, error)

type NoRequest struct{}

// NoResponse is answered with 204.
type NoResponse struct{}

type PathBindable interface{ BindPath(r *http.Request) }
type QueryBindable interface{ BindQuery(r *http.Request) }
type Validator interface{ Validate() error }

// RequestBindable takes what the other binders cannot reach: cookies, headers, the remote address.
type RequestBindable interface{ BindRequest(r *http.Request) }

// BodyBindable reads the body itself, instead of the JSON decoding of the adapter. Use it for a raw body or a bigger
// limit than 16 KiB. An *ErrorResponse it returns is answered as it is; any other error is a 400 bad_json.
type BodyBindable interface{ BindBody(r *http.Request) error }

// BodyFree marks a request that has no body, although its method (POST, PUT) usually has one.
type BodyFree interface{ BodyFree() }

// StatusCoder is a response that is not answered with 200. A 204 is sent without a body.
type StatusCoder interface{ StatusCode() int }

// CookieSetter is a response that sets cookies.
type CookieSetter interface{ SetCookies() []*http.Cookie }

// ErrorResponse is an error that carries its HTTP status and the machine-readable code of the body.
type ErrorResponse struct {
	Status int
	Code   string
	Err    error
}

func (e *ErrorResponse) Error() string { return e.Err.Error() }
func (e *ErrorResponse) Unwrap() error { return e.Err }

var defaultCodes = map[int]string{
	http.StatusBadRequest:   "invalid",
	http.StatusUnauthorized: "unauthenticated",
	http.StatusForbidden:    "forbidden",
	http.StatusNotFound:     "not_found",
	http.StatusConflict:     "conflict",
}

func NewError(statusCode int, err error) *ErrorResponse {
	code, ok := defaultCodes[statusCode]
	if !ok {
		code = "internal"
	}

	return NewErrorWithCode(statusCode, code, err)
}

func NewErrorWithCode(statusCode int, code string, err error) *ErrorResponse {
	return &ErrorResponse{Status: statusCode, Code: code, Err: err}
}

const maxBody = 16 << 10

// Authenticator answers whether the request may be served at all. It runs before anything else is read from the request,
// so a stranger gets the same error whatever they send.
type Authenticator interface {
	Authenticate(ctx context.Context) error
}

// AdaptEndpoint decodes the JSON body of a POST, PUT or PATCH (unless Req is NoRequest), then binds the path and the
// query, validates, and runs ep. An *ErrorResponse anywhere in the chain sets the status; any other error is a 500 whose
// detail is only logged.
func AdaptEndpoint[Req any, Resp any](ep Endpoint[Req, Resp]) http.HandlerFunc {
	return AdaptAuthenticatedEndpoint(nil, ep)
}

// AdaptAuthenticatedEndpoint is AdaptEndpoint with auth checked first: its error is answered before the body is decoded.
func AdaptAuthenticatedEndpoint[Req any, Resp any](auth Authenticator, ep Endpoint[Req, Resp]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if auth != nil {
			if err := auth.Authenticate(r.Context()); err != nil {
				respondError(w, err)
				return
			}
		}
		var req Req
		if b, ok := any(&req).(BodyBindable); ok {
			if err := b.BindBody(r); err != nil {
				var oe *ErrorResponse
				if !errors.As(err, &oe) {
					oe = NewErrorWithCode(http.StatusBadRequest, "bad_json", errors.New("body is not valid JSON"))
				}
				writeError(w, oe.Status, oe.Code, oe.Err.Error())

				return
			}
		} else if !bodyFree(&req) && hasBody(r.Method) {
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "bad_json", "body is not valid JSON")
				return
			}
		}
		if b, ok := any(&req).(PathBindable); ok {
			b.BindPath(r)
		}
		if b, ok := any(&req).(QueryBindable); ok {
			b.BindQuery(r)
		}
		if b, ok := any(&req).(RequestBindable); ok {
			b.BindRequest(r)
		}
		if v, ok := any(&req).(Validator); ok {
			if err := v.Validate(); err != nil {
				var oe *ErrorResponse
				if !errors.As(err, &oe) {
					oe = NewError(http.StatusBadRequest, err)
				}
				writeError(w, oe.Status, oe.Code, oe.Err.Error())

				return
			}
		}
		resp, err := ep(r.Context(), &req)
		if err != nil {
			respondError(w, err)
			return
		}

		if _, none := any(resp).(*NoResponse); none {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if c, ok := any(resp).(CookieSetter); ok {
			for _, cookie := range c.SetCookies() {
				http.SetCookie(w, cookie)
			}
		}
		status := http.StatusOK
		if c, ok := any(resp).(StatusCoder); ok {
			status = c.StatusCode()
		}
		if status == http.StatusNoContent {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// respondError answers an *ErrorResponse below 500 as it is; anything else is a 500 whose detail is only logged.
func respondError(w http.ResponseWriter, err error) {
	var oe *ErrorResponse
	if errors.As(err, &oe) && oe.Status < 500 {
		writeError(w, oe.Status, oe.Code, oe.Err.Error())
		return
	}
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal", "internal error")
}

// bodyFree reports whether no JSON is to be decoded into req.
func bodyFree(req any) bool {
	if _, ok := req.(*NoRequest); ok {
		return true
	}
	_, ok := req.(BodyFree)

	return ok
}

func hasBody(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg, "code": code})
}
