// Package validate runs the engine's validate-puzzle tool.
//
// The solver lives in the web repo (one engine, no copy that can drift). The API treats it as
// untrusted work: separate process, hard timeout, output size cap.
package validate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// Verdict is the tool's stdout. Keep in step with contract/validate-verdict.md.
type Verdict struct {
	Valid     bool     `json:"valid"`
	Solutions string   `json:"solutions"` // "none" | "one" | "many"
	Errors    []string `json:"errors,omitempty"`
}

type Validator interface {
	Validate(ctx context.Context, puzzle []byte) (Verdict, error)
}

// Accept approves everything. Dev only.
type Accept struct{}

func (Accept) Validate(context.Context, []byte) (Verdict, error) {
	return Verdict{Valid: true, Solutions: "one"}, nil
}

// Exec runs Path with the puzzle JSON on stdin and reads a Verdict from stdout.
type Exec struct {
	Path    string
	Timeout time.Duration
}

const maxOutput = 64 << 10

func (e Exec) Validate(ctx context.Context, puzzle []byte) (Verdict, error) {
	runCtx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, e.Path)
	cmd.Stdin = bytes.NewReader(puzzle)
	var out bytes.Buffer
	// The tool is untrusted: cap what it can make us buffer.
	cmd.Stdout = &limitedWriter{w: &out, n: maxOutput}

	if err := cmd.Run(); err != nil {
		return Verdict{}, fmt.Errorf("validate-puzzle: %w", err)
	}

	var v Verdict
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		return Verdict{}, fmt.Errorf("validate-puzzle output: %w", err)
	}

	return v, nil
}

type limitedWriter struct {
	w *bytes.Buffer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.w.Len()+len(p) > l.n {
		return 0, fmt.Errorf("output over %d bytes", l.n)
	}

	return l.w.Write(p)
}
