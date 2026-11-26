package sudokupad

import (
	"context"
	"errors"
)

type Endpoint struct {
	limitIP     Limiter
	limitGlobal Limiter
	source      Source
	trustProxy  bool
}

func New(limitIP, limitGlobal Limiter, source Source, trustProxy bool) *Endpoint {
	return &Endpoint{limitIP: limitIP, limitGlobal: limitGlobal, source: source, trustProxy: trustProxy}
}

type Limiter interface {
	Allow(key string) bool
}

// Source reads the puzzle data of a short SudokuPad id. It returns ErrNotFound when the site has no puzzle for the id.
type Source interface {
	Fetch(ctx context.Context, id string) (data string, err error)
}

// ErrNotFound is the answer of a Source for an id without a puzzle.
var ErrNotFound = errors.New("sudokupad.app has no puzzle for this id")
