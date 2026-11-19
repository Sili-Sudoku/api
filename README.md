# Sili Sudoku API

Backend for [Sili Sudoku](../web). Publishing, accounts, community and solve history all run on Postgres (CockroachDB works too); set `DATABASE_URL`.

It is four programs, one per area, behind one address that routes by path:

| Program | Serves | Default port (dev) |
|---|---|---|
| `cmd/accounts` | sign-up, sessions, `/v1/me` | 8081 |
| `cmd/puzzles` | publish, browse, get, claim, report, delete, and the background validation | 8082 |
| `cmd/community` | ratings, comments, profiles, follows, feed, solve stats | 8083 |
| `cmd/history` | `/v1/me/history` | 8084 |

`cmd/migrate` applies `migrations/` and exits; run it before the programs. `deploy/Caddyfile` is the local front door (`:8080`) and holds the routing rules; the cloud load balancer needs the same ones. `STYLE.md` has the table and `integration/programs_test.go` checks it against `contract/openapi.yaml`.

    cd cmd/migrate && DATABASE_URL=postgres://... go run .
    cd cmd/accounts && DATABASE_URL=postgres://... API_ADDR=:8081 go run .    # and puzzles, community, history on their ports
    TEST_DATABASE_URL=postgres://... go test -race github.com/Sili-Sudoku/api/...   # without the variable, the database tests are skipped

`../dev.sh` starts the whole stack (Postgres, the programs, Caddy, seed data, Vite) on the host.

## Running in the cloud
- One image per program: `docker build --build-arg SERVICE=<program> .`. Not tried yet: the claude-box cannot build images.
- The `puzzles` service runs the publish worker and a 30 s sweep, so it needs CPU always allocated and at least one instance (Cloud Run pauses CPU after a response and scales to zero). Its image also needs the `validate-puzzle` tool from the web repo and Node 24 (`VALIDATE_CMD`).
- Rate limits are in-process, so they are per instance.
- Run `migrate` as a job before a deploy, not at startup.

See `CLAUDE.md` for the layout and rules.
