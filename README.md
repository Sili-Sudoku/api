# Sili Sudoku API

Backend for [Sili Sudoku](https://github.com/Sili-Sudoku/dashboard). Publishing, accounts, community and solve history all run on Postgres (CockroachDB works too); set `DATABASE_URL`.

It is four programs, one per area, behind one address that routes by path:

| Program | Serves | Default port (dev) |
|---|---|---|
| `services/accounts` | sign-up, sessions, `/v1/me` | 8081 |
| `services/puzzles` | publish, browse, get, claim, report, delete, and the background validation | 8082 |
| `services/community` | ratings, comments, profiles, follows, feed, solve stats | 8083 |
| `services/history` | `/v1/me/history` | 8084 |

`tools/migrate` applies `migrations/` and exits; run it before the programs. `deploy/Caddyfile` is the local front door (`:8080`) and holds the routing rules; the cloud load balancer needs the same ones. `STYLE.md` has the table and `integration/programs_test.go` checks it against `contract/openapi.yaml`.

    cd tools/migrate && DATABASE_URL=postgres://... go run .
    cd services/accounts && DATABASE_URL=postgres://... API_ADDR=:8081 go run .    # and puzzles, community, history on their ports
    TEST_DATABASE_URL=postgres://... go test -race github.com/Sili-Sudoku/api/...   # without the variable, the database tests are skipped


## Running in the cloud
- One image per program, from the `main.dockerfile` next to its main: `docker build -f services/<program>/main.dockerfile .` (and `tools/migrate/main.dockerfile`), run in this directory. They are generated from each `go.work`; `libs/boundary` fails when one is out of date (`UPDATE_DOCKERFILES=1` rewrites them, see `STYLE.md`).
- The `puzzles` service validates inside the publish request (and again on a read of a still-pending puzzle), so it needs no background CPU and scales to zero. It runs at most 3 engine processes at once. Its image also needs the `validate-puzzle` tool from the web repo and Node 24 (`VALIDATE_CMD`).
- Rate limits are in-process, so they are per instance.
- Run `migrate` as a job before a deploy, not at startup.

See `CLAUDE.md` for the layout and rules.

## Contributing, security and license

- Read [`CONTRIBUTING.md`](CONTRIBUTING.md) before you open a pull request.
- To report a security problem, read [`SECURITY.md`](SECURITY.md). Do not open a public issue.
- The code is under the [Business Source License 1.1](LICENSE), the same license as the web app. You can read, run and
  change it, and you can use it in production. You must not offer it, or a derivative, as a hosted puzzle service that
  competes with the licensor. Each version changes to Apache-2.0 four years after its first public release. For other
  terms, write to `contact@silical.dev`.
