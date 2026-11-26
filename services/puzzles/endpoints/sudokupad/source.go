package sudokupad

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Safety: bounds on what the upstream can cost a request.
const (
	fetchTimeout = 15 * time.Second
	maxDataBytes = 1 << 20
)

// HTTPSource reads puzzles from the sudokupad.app API.
type HTTPSource struct {
	Client  *http.Client
	BaseURL string // the id is appended
}

func NewHTTPSource() *HTTPSource {
	return &HTTPSource{
		Client: &http.Client{
			Timeout: fetchTimeout,
			// Safety: never follow a redirect, so the request cannot be sent to another host.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		BaseURL: "https://sudokupad.app/api/puzzle/",
	}
}

func (s *HTTPSource) Fetch(ctx context.Context, id string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.BaseURL+id, nil)
	if err != nil {
		return "", fmt.Errorf("make the request: %w", err)
	}
	req.Header.Set("User-Agent", "sili-sudoku-import")

	resp, err := s.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sudokupad.app answered HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDataBytes+1))
	if err != nil {
		return "", fmt.Errorf("read the answer: %w", err)
	}

	if len(body) > maxDataBytes {
		return "", errors.New("the answer is over 1 MiB")
	}

	// Anything else (an empty answer, an HTML page) is not a puzzle.
	if !strings.HasPrefix(string(body), "scl") {
		return "", ErrNotFound
	}

	return string(body), nil
}
