package publish

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/Sili-Sudoku/api/libs/clientip"
	"github.com/Sili-Sudoku/api/libs/http_outcome"
	st "github.com/Sili-Sudoku/api/libs/stack_trace"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

// ValidateTimeout bounds the validation inside a publish request.
const ValidateTimeout = 60 * time.Second

// Text limits. They match LIMITS in the web repo's src/runtime/serialization.ts.
const (
	maxTitle, maxAuthor, maxSource, maxDifficulty, maxTag, maxTags = 200, 100, 500, 50, 50, 30
)

// EndpointReq is the puzzle as received: the id is the hash of these exact bytes.
type EndpointReq struct {
	Body []byte

	remoteAddr   string
	forwardedFor string
}

type EndpointResp struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	DeleteToken string `json:"deleteToken,omitempty"` // shown once
}

func (this *EndpointResp) StatusCode() int { return http.StatusAccepted }

func (this *EndpointReq) BindBody(req *http.Request) error {
	body, err := io.ReadAll(http.MaxBytesReader(nil, req.Body, model.MaxPuzzleBytes))
	if err != nil {
		return http_outcome.NewErrorWithCode(http.StatusRequestEntityTooLarge, "too_large", errors.New("puzzle too large"))
	}

	if !json.Valid(body) {
		return http_outcome.NewErrorWithCode(http.StatusBadRequest, "bad_json", errors.New("body is not JSON"))
	}
	this.Body = body

	return nil
}

func (this *EndpointReq) BindRequest(req *http.Request) {
	this.remoteAddr, this.forwardedFor = req.RemoteAddr, req.Header.Get("X-Forwarded-For")
}

func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *EndpointResp, err error) {
	user := this.auth.FindUser(ctx)
	key := "ip:" + clientip.Resolve(req.remoteAddr, req.forwardedFor, this.trustProxy)
	var ownerID int64
	if user != nil {
		key, ownerID = "user:"+user.Username, user.ID
	}
	if !this.limit.Allow(key) {
		return nil, http_outcome.NewErrorWithCode(http.StatusTooManyRequests, "rate_limited", errors.New("too many puzzles published; try again later"))
	}

	sum := sha256.Sum256(req.Body)
	id := base64.RawURLEncoding.EncodeToString(sum[:12])
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, st.StackTrace(rand.Read, err)
	}

	token := base64.RawURLEncoding.EncodeToString(raw)

	p := &model.Puzzle{ID: id, Body: req.Body, Status: model.Pending, Moderation: model.ModNone, OwnerID: ownerID,
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond), TokenHash: sha256.Sum256([]byte(token))}
	if err := describe(req.Body, p); err != nil {
		return nil, http_outcome.NewErrorWithCode(http.StatusBadRequest, "invalid_puzzle", err)
	}

	created, err := this.db.Puzzle.Create(ctx, p)
	if err != nil {
		return nil, st.StackTrace(this.db.Puzzle.Create, err)
	}

	// The engine runs here, inside the request, because a host that only gives CPU to requests (Cloud Run) would starve a
	// background job. The client leaving does not stop it. The answer stays 202 "pending" and the client reads the result:
	// if the engine failed, the puzzle stays pending and the next read of it tries again.
	vctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ValidateTimeout)
	defer cancel()
	if err := this.validator.Validate(vctx, id); err != nil {
		log.Printf("validate %s: %v", id, err)
	}

	resp = &EndpointResp{ID: id, Status: string(model.Pending)}
	// The same bytes always give the same id, so a repeat publish returns no token; an owner needs none.
	if created && ownerID == 0 {
		resp.DeleteToken = token
	}

	return resp, nil
}

// meta is the part of a puzzle that lists show. The engine, not this code, decides if the puzzle is valid.
type meta struct {
	Metadata struct {
		Title      string   `json:"title"`
		Author     string   `json:"author"`
		Source     string   `json:"source"`
		Difficulty string   `json:"difficulty"`
		Tags       []string `json:"tags"`
	} `json:"metadata"`
	Constraints []json.RawMessage `json:"constraints"`
}

func tooLong(s string, max int) bool { return utf8.RuneCountInString(s) > max }

func invalid(why string) error { return errors.New("invalid puzzle: " + why) }

// describe copies list fields out of the body. It checks types and lengths only.
func describe(body []byte, p *model.Puzzle) error {
	var m meta
	if err := json.Unmarshal(body, &m); err != nil {
		return invalid("metadata has the wrong shape")
	}

	md := m.Metadata
	if tooLong(md.Title, maxTitle) || tooLong(md.Author, maxAuthor) || tooLong(md.Source, maxSource) ||
		tooLong(md.Difficulty, maxDifficulty) || len(md.Tags) > maxTags {
		return invalid("a text field is too long")
	}
	for _, t := range md.Tags {
		if tooLong(t, maxTag) {
			return invalid("a tag is too long")
		}
	}
	if md.Source != "" {
		if u, err := url.Parse(md.Source); err != nil || u.Scheme != "https" || u.Host == "" {
			return invalid("source must be an https link")
		}
	}
	p.Title, p.Author, p.Source, p.Difficulty = md.Title, md.Author, md.Source, md.Difficulty
	p.Tags = append([]string{}, md.Tags...)
	seen := map[string]bool{}
	p.Variants = []string{}
	for _, c := range m.Constraints {
		var use struct {
			Use string `json:"use"`
		}
		if json.Unmarshal(c, &use) == nil && use.Use != "" && !seen[use.Use] && len(use.Use) <= 50 {
			seen[use.Use] = true
			p.Variants = append(p.Variants, use.Use)
		}
	}
	sort.Strings(p.Variants)

	return nil
}
