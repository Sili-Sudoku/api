# Sili Sudoku API

Go backend for the Sili Sudoku web app (sibling repo `../web`). Postgres via pgx (CockroachDB works too, `DATABASE_URL` is required); otherwise standard library.

## Style
Follow `STYLE.md` (the ws2wh conventions). It overrides the "Map" below wherever they disagree; update the Map as areas are converted.
Follow `CONVENTIONS.md` for how the code inside is laid out (spacing, structs, names, comments).

## Check before you finish
    gofmt -l . && go vet github.com/Sili-Sudoku/api/... && go test -race github.com/Sili-Sudoku/api/...

`api/` is many Go modules (a `go.mod` per lib, service, model, postgres client and program) with a `go.work` in each, so `./...` does not work from `api/`; use the import path above, or `cd` into one module and run `go test ./...` there. See `STYLE.md`, Modules.

The store tests and the `server` HTTP tests need a database and are skipped unless `TEST_DATABASE_URL` is set (the endpoint unit tests use mocks and always run). Store tests run on a transaction that is always rolled back (`dbtest.Tx`), so they leave nothing behind and packages can run in parallel. The schema is migrated on first use and kept. A statement that fails aborts the transaction (25P02): run it in a savepoint (`tx.Begin`) or last. The `server` HTTP tests need committed data (the publish worker uses its own connection), so they use `dbtest.Pool`: a database named like yours plus `_commit`, emptied around each test, one test at a time per package. Use a database of your own, not one with data you want to keep.

The box's shared dev CockroachDB works too (v26, the code is tested on it). Use a database of its own:

    cockroach sql --insecure --host=cockroach:26257 -e "create database if not exists sudoku_test"
    TEST_DATABASE_URL='postgresql://root@cockroach:26257/sudoku_test?sslmode=disable' go test -race github.com/Sili-Sudoku/api/...

Or a throwaway Postgres:

    podman run -d --rm --name sudoku-pg-test -e POSTGRES_PASSWORD=test -e POSTGRES_DB=sudoku -p 127.0.0.1:55432:5432 docker.io/library/postgres:17
    TEST_DATABASE_URL=postgres://postgres:test@127.0.0.1:55432/sudoku go test -race github.com/Sili-Sudoku/api/...

## Rules
- Local-first: the web app must work with this API down. Never add a route the app needs to solve, author or import.
- Puzzles are untrusted input. Cap sizes, never render or trust free text, validate with the engine tool (`libs/validate`), never with a second solver written here.
- Publishing is asynchronous: store as `pending`, validate in a background job, then `published` or `rejected`. Never run the validator inside a request.
- Anonymous publish comes first (content-addressed id + secret delete token). Accounts add an owner later.
- Keep secrets out of the repo. Config is env vars; see `.env.example`.

## Map
| Area | Package | Board card(s) | State |
|---|---|---|---|
| Publish, list, get, claim, report, delete | `services/puzzles` (`endpoints/{publish,list,getpuzzle,claim,report,deletepuzzle}`, `postgres`, `worker`, `model`) | #241 report/delete + anonymous publish; #57 discovery; #219 sharing | working, Postgres only |
| Engine validation | `libs/validate`, `services/puzzles/worker` | Surface uniqueness at publish; solver | working (exec + timeout; the worker runs it in the background and from a 30 s sweep) |
| Signup, login, logout, me, bio, delete account | `services/accounts` (`endpoints/{signup,login,logout,getme,patchme,deleteme}`, `postgres`, `session`, `model`), `libs/password` | Community: accounts (#56) | working, Postgres only |
| Rate limits | `libs/ratelimit` (each area's `routes` package owns its limits) | | working (in-process) |
| Ratings, felt difficulty, comments, solve-time stats | `services/community` (`endpoints/{getratings,putrating,listcomments,postcomment,deletecomment,reportcomment,solvestats}`, `postgres`, `model`) | #58 | working, Postgres only |
| Profiles, follows, feed | `services/community` (`endpoints/{getprofile,follow,unfollow,listfollowing,feed}`) | #59 | working, Postgres only |
| Solve history (best time per puzzle, opt-in) | `services/history` (`endpoints/{record,list,deleteall}`, `postgres`, `model`) | Solve timer/stats (local first) | working, Postgres only |
| Shared plumbing | `libs/{http_outcome,stack_trace,cursor,clientip,password,mockprovider}` | | adapter, error wrapping, test runners and mocks |
| Browse (`GET /v1/puzzles`, filters, cursor) | `services/puzzles/endpoints/list` | #57 Publishing and discovery | working (`sort=newest` or `top`; top pages by offset) |
| Routes and middleware | `services/<area>/routes` (`Deps`, `New`), `libs/server` | | each area wires its own endpoints; `libs/server` holds only what all share: CSRF check, session loading, the `{error, code}` body, `Auth`, `Config`. `integration` runs the four areas together in tests |
| Admin tool | `cmd/admin` | moderation (hide-at-3) | working: promote, demote, hide, unhide, delete, hide-comment, unhide-comment, delete-comment |
| Programs | `cmd/{accounts,puzzles,community,history}`, `cmd/migrate`, `deploy/Caddyfile`, `Dockerfile` | | working locally: one program per area behind Caddy (see `STYLE.md`, Deployables); `libs/boot` is the shared plumbing; images and the cloud load balancer are not tried yet |
| Layout rules | `libs/boundary` | | enforced by `go test`: who may import whom (libs, services, programs), and that each module's `go.work` lists exactly the sibling modules its code needs (`TestWorkspaces`). A new cross-service import fails the test; change the rule in `boundary_test.go` on purpose and note the reason in `STYLE.md` |
| Dev seed | `cmd/seed` | | working (two users, bundled puzzles) |
| Schema | `migrations/` | | `0001_puzzles.sql`, `0002_accounts.sql`, `0003_puzzle_listing.sql`, `0004_community.sql`, `0005_history.sql`, `0006_follows.sql` applied by `cmd/migrate`; other tables drafted in `migrations/draft/schema.sql` |

## Contract with `../web`
`contract/` holds the OpenAPI file and the PuzzleDefinition JSON Schema. They are generated in the web repo and copied here; do not hand-edit them. See `contract/README.md`.
Every route in the OpenAPI file is built; there are no stub routes left.

## Accounts
Username + password (argon2id), server-side sessions, cookie `session` (HttpOnly, Secure, SameSite=Lax). Every POST/PUT/PATCH/DELETE needs `X-Requested-With: sudoku`. Errors are `{error, code}`. Rate limits are in `server.NewLimits`. A user is `is_admin` only by a direct database change (`cmd/admin`, see below once built).

## Community
Ratings: one row per user and puzzle, `stars` and `difficulty` (felt) each 0 or 1-5. After each vote the endpoint recomputes the totals from the rows and writes them on the puzzle (`rating_*`, `diff_*`), so `sort=top` needs no join. The top score is `(sum+6)*1000/(count+2)`. You cannot rate your own puzzle.
Comments: plain text, 1-2000 characters, newest first, keyset cursor. Signed in to post, report (not your own) or delete (own, or admin). 3 reports hide a comment until `cmd/admin unhide-comment`. Comment rate limit: `Limits.Comment` per user.

## History and profiles
History is opt-in on the client; the server stores one row per user and puzzle: the best time and when it was set (no progress snapshots yet). `PUT /v1/me/history/{id}` merges (the lower time wins), `solve_history` has no foreign key to puzzles, and `history/postgres.SolveHistory.SolveSeconds` gives the `solvestats` endpoint the times. 5000 puzzles per user at most.
Profiles are `GET /v1/users/{name}` (bio, counts, whether you follow). Follow is idempotent, not yourself, 500 creators at most. `GET /v1/me/feed` is the list of puzzles filtered to followed creators (`puzzles.Filter.OwnerIDs`).

## Not in the MVP (follow-ups)
Achievements and badges; analytics; thumbnails; email verification and password reset; OAuth and passkeys; notifications; an admin UI; moderation beyond hide-at-3; Penpa+ import and export.

## Open decisions
- Migrations: plain SQL files applied in name order by `cmd/migrate` (`libs/db`). Add a new numbered file for each change; never edit an applied one. Move tables out of `migrations/draft/` as their area gets built.
- Moderation model for comments: MVP is hide-at-3-reports plus `cmd/admin` (see Phase 5 notes).
- Licence for puzzle content (board card).
