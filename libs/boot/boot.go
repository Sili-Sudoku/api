// Package boot is the plumbing every program shares: the database pool, the validator and the HTTP server loop. Each
// program builds only the clients it uses.
package boot

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Sili-Sudoku/api/libs/db"
	"github.com/Sili-Sudoku/api/libs/validate"
)

// OpenPool opens the database from DATABASE_URL (required). It applies no migrations: run tools/migrate first.
func OpenPool(ctx context.Context) (*db.Pool, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}

	return db.Open(ctx, url)
}

// Validator reads VALIDATE_CMD (empty = accept every puzzle, dev only). With REQUIRE_VALIDATOR=true an empty VALIDATE_CMD
// stops the program, so a deploy that lost the tool fails at once and never accepts unchecked puzzles.
func Validator() validate.Validator {
	cmd := os.Getenv("VALIDATE_CMD")
	if err := checkValidator(cmd, os.Getenv("REQUIRE_VALIDATOR") == "true"); err != nil {
		log.Fatal(err)
	}

	if cmd == "" {
		log.Print("VALIDATE_CMD is empty: accepting every puzzle (dev only)")
		return validate.Accept{}
	}

	return validate.Exec{Path: cmd, Timeout: 20 * time.Second}
}

// Serve listens on API_ADDR (default :8080) and does not return unless the server stops. If CLIENT_IP_HEADER is set, the
// address in that header becomes the client address (see clientIPFromHeader).
func Serve(name string, h http.Handler) {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	if header := os.Getenv("CLIENT_IP_HEADER"); header != "" {
		h = clientIPFromHeader(header, h)
		log.Printf("%s reads the client address from %s", name, header)
	}

	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("%s listening on %s", name, addr)
	log.Fatal(srv.ListenAndServe())
}

// clientIPFromHeader makes the address in header the only X-Forwarded-For entry. Use it when a private service sits behind
// a gateway that puts the real client address in that header (the Cloudflare Worker does): Cloud Run adds the address it
// saw, the gateway's, to the end of X-Forwarded-For, so TRUST_PROXY alone would rate-limit every user as one. A request
// without the header keeps no X-Forwarded-For at all, so a forged chain is never trusted.
func clientIPFromHeader(header string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := strings.TrimSpace(r.Header.Get(header)); ip != "" {
			r.Header.Set("X-Forwarded-For", ip)
		} else {
			r.Header.Del("X-Forwarded-For")
		}

		next.ServeHTTP(w, r)
	})
}

func checkValidator(cmd string, required bool) error {
	if required && cmd == "" {
		return errors.New("REQUIRE_VALIDATOR is true but VALIDATE_CMD is empty")
	}

	return nil
}
