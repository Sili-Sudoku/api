package sudokupad

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"

	"github.com/Sili-Sudoku/api/libs/clientip"
	"github.com/Sili-Sudoku/api/libs/http_outcome"
)

// idPattern is the short id of a sudokupad.app link. It is the same as the Vite dev proxy of the web repo.
// Safety: the id goes into the upstream URL, so nothing outside these characters may pass.
var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{4,40}(/[A-Za-z0-9_-]{2,40})?$`)

type EndpointReq struct {
	ID string

	remoteAddr   string
	forwardedFor string
}

type EndpointResp struct {
	Data string `json:"data"` // the scl data, as sudokupad.app gave it
}

func (this *EndpointReq) BindQuery(req *http.Request) {
	this.ID = req.URL.Query().Get("id")
}

func (this *EndpointReq) BindRequest(req *http.Request) {
	this.remoteAddr, this.forwardedFor = req.RemoteAddr, req.Header.Get("X-Forwarded-For")
}

// Execute fetches a puzzle from sudokupad.app for a browser that cannot (the site sends no CORS headers). The endpoint
// is open, so it has two limits: one per client address, and one for everybody, which protects sudokupad.app.
func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	if !idPattern.MatchString(req.ID) {
		return nil, http_outcome.NewErrorWithCode(http.StatusBadRequest, "bad_id", errors.New("not a SudokuPad puzzle id"))
	}

	ip := clientip.Resolve(req.remoteAddr, req.forwardedFor, this.trustProxy)
	if !this.limitIP.Allow("ip:" + ip) {
		return nil, http_outcome.NewErrorWithCode(http.StatusTooManyRequests, "rate_limited", errors.New("too many imports; try again later"))
	}

	if !this.limitGlobal.Allow("global") {
		return nil, http_outcome.NewErrorWithCode(http.StatusTooManyRequests, "busy",
			errors.New("too many imports from everybody right now; try again later, or paste the puzzle data"))
	}

	data, err := this.source.Fetch(ctx, req.ID)
	if errors.Is(err, ErrNotFound) {
		return nil, http_outcome.NewErrorWithCode(http.StatusNotFound, "not_found", err)
	}

	if err != nil {
		log.Printf("sudokupad fetch %q: %v", req.ID, err)

		return nil, http_outcome.NewErrorWithCode(http.StatusBadGateway, "upstream", errors.New("cannot read sudokupad.app right now"))
	}

	return &EndpointResp{Data: data}, nil
}
