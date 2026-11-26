# API style: follow the ws2wh conventions

The user's other Go backend (`ws2wh`) is the house style. The box cannot see that repo, so the patterns are copied here.
Import paths start with `github.com/Sili-Sudoku/api/`. It is many Go modules tied together by `go.work` files, like ws2wh; see "Modules".
The areas are deployed as separate programs, see "Deployables".

Work in a **pilot first**: convert `services/history` only, run `gofmt -l . && go vet github.com/Sili-Sudoku/api/... && go test -race github.com/Sili-Sudoku/api/...`, then stop and
report so the user can review the style before the rest is converted. Report anything that does not fit; ask before inventing.

## Modules: a go.mod per piece, a go.work per module (decided by the user)
Why: a service must not carry a dependency it does not use. When accounts gets Google login, the SDK must not reach puzzles,
community or history.
- A module is a directory with a `go.mod` whose path is `github.com/Sili-Sudoku/api/<directory>`, so import paths do not change
  when modules are cut differently. The pieces: every `libs/<lib>`; every `services/<area>`, plus `services/<area>/model`
  and `services/<area>/postgres` as modules of their own (other areas import only those two, never the endpoints), and
  `services/accounts/session` (the session check that `libs/server` uses); `tools/<tool>`; `migrations`; `integration`. The main package of a program is at the root of its `services/<area>` module.
- `go.mod` lists outside dependencies only (pgx, x/crypto). It never requires a sibling module. Run `go work sync` to fill in
  the indirect ones.
- `go.work` (in each module) lists the module itself and every sibling it builds with, directly or through another module,
  tests included: `use ( . ../../libs/db ... )`. Go finds the nearest `go.work`, so building inside a module sees only its own.
- The root `api/go.work` lists all modules. It is for working across them: `go test` and `go vet` run from `api/`, and the root
  is not a module, so `./...` does not work there. Use the import path: `go test -race github.com/Sili-Sudoku/api/...`.
- Adding a module: create its `go.mod` and `go.work`, add it to the root `go.work`, add it to the `go.work` of every module
  that imports it and of every module that imports those. `libs/boundary` (`TestWorkspaces`) fails with the exact lines that are
  missing or extra, so run it after any import or module change.
- Each program has a `main.dockerfile` next to its main (as in ws2wh). It copies the `go.mod` of every module in the program's `go.work`, downloads, then copies those modules and builds, so the image only sees that program's modules and the download layer is cached. The files are generated: `TestDockerfiles` fails when one is out of date with its `go.work` or `go.sum` files, and `UPDATE_DOCKERFILES=1 go test github.com/Sili-Sudoku/api/libs/boundary -run Dockerfiles` rewrites them.

## Deployables: one program per area (decided by the user)
Each area is its own program, packaged as its own container and run as one Cloud Run service; locally the same containers sit
on one network group. One `services/<area>/main.go` per program (as in ws2wh, the main sits with its service), one `main.dockerfile` each. Programs: `accounts`, `puzzles`, `community`, `history`. `tools/admin`, `tools/migrate` and `tools/seed` are the tools. Isolation comes from separate binaries and images, and from separate modules (below), so a dependency
one area needs is not carried by the others.

- A program imports its own service, `libs`, and only what it must share. Known shares: every program checks the session
  through `services/accounts/session` (so `libs/server` imports `services/accounts`; `libs/mockprovider` imports service models too: libs depending on services is a known exception); `community` reads solve times from `history/postgres` and writes the `rating_*` columns on
  `puzzles`; `accounts` deletion touches the other areas' rows in one transaction; `history` reads puzzle titles through
  `puzzles/postgres` `Find` (the list). Keep that list short and write new ones here.
- Each program builds only the clients it uses. `libs/boot` shrinks to shared plumbing (config, pool); it no longer builds one
  `Stack` of everything.
- Routes and response shapes do not change; the web app must not notice. Because one URL prefix spans areas, the proxy
  (local: Caddy; cloud: the load balancer's path rules) routes by method and path. Match the specific rows before the general ones:

      accounts   POST /v1/accounts   POST /v1/sessions   DELETE /v1/sessions/current   GET|PATCH|DELETE /v1/me
      history    /v1/me/history   /v1/me/history/{puzzleId}                  (before /v1/me)
      community  /v1/me/following  /v1/me/feed  /v1/users/{name}  /v1/users/{name}/follow  /v1/comments/{id}  /v1/comments/{id}/report
                 /v1/puzzles/{id}/ratings  /v1/puzzles/{id}/comments  /v1/puzzles/{id}/stats       (before /v1/puzzles)
      puzzles    /v1/puzzles  /v1/puzzles/{id}  /v1/puzzles/{id}/owner  /v1/puzzles/{id}/report

- Migrations do not run at startup of four programs at once. Make `tools/migrate` (a Cloud Run job, and a one-off command locally)
  and have it apply the files under an advisory lock.
- Cloud Run only gives CPU to requests and scales to zero. Nothing may run between requests: do work inside the request (as
  `puzzles` does for validation) or in a scheduled job, never in a background goroutine or sweep loop. Rate limits
  are in-process, so they are per instance; fine for the MVP, note it.
- Status: done locally. `services/{accounts,puzzles,community,history}/main.go` and `tools/migrate` exist, `libs/boot` is only the pool, validator and
  server loop, `deploy/Caddyfile` routes by path, and `integration/programs_test.go` checks the routing table against the contract and the Caddyfile.
  The dockerfiles of `history` and `puzzles` build on the host; the cloud load balancer is not tried.

- The import rules above are enforced, not just written: `libs/boundary` runs in `go test ./...` and fails when a package
  imports where it must not (an endpoint importing another service, a model importing its postgres client, a lib importing
  an endpoint, a program importing another area's routes). Its rules and its two lib exceptions live in `boundary_test.go`. The same package checks the go.work files (see Modules).
  If a new share is deliberate, edit the rule there and write the reason here; do not work around the test.

## Keep (these come from the sudoku rules, not taste)
Cookie sessions and the `X-Requested-With` CSRF check, the async publish worker (never run the validator inside a request),
content-addressed puzzle ids, argon2id with the dummy hash, the OpenAPI contract, the `{error, code}` error body, and every
existing route and response shape. The web app must not notice the refactor.

## Remove
- Every `memstore.go`, the `MemoryStore` paths in the old `cmd/api` and `tools/seed`, and the memory-mode e2e setup. Report what depended on them first.
- Store tests run against the shared dev CockroachDB (see `CLAUDE.md`) with a database of its own. Unit tests mock each
  endpoint's own small interfaces. Run the existing store tests on Cockroach early; the SQL has never run, so expect failures.

## Layout: one package per endpoint
    services/<area>/endpoints/<action>/handler.go       Endpoint struct, New, the interfaces it needs
    services/<area>/endpoints/<action>/service.go       EndpointReq (Validate, BindPath, BindQuery) and Execute
    services/<area>/endpoints/<action>/service_test.go
    services/<area>/postgres/...                        one client per table (pgx), Find returns nil, nil
    services/<area>/routes/routes.go                    the area's Deps (its clients, Config, Limits), New(Deps) http.Handler and the mux wiring
    libs/server                                         only what every area shares: CSRF check, session loading, {error, code}, Auth, Config
    integration                                         tests that run the four areas together on one database
Do not name a package after a Go builtin (`delete`, `new`, `copy`): use `deletepuzzle` and so on.

handler.go: dependencies are interfaces declared by the consumer, listed alphabetically.

    package create

    type Endpoint struct {
        auth Auth
        db   *DB
    }

    func New(auth Auth, db *DB) *Endpoint { return &Endpoint{auth: auth, db: db} }

    type Auth interface {
        GetIdentityID(ctx context.Context) (model.IdentityID, error)
    }

    type DB struct {
        WebsocketConnection WebsocketConnectionDB
    }

    type WebsocketConnectionDB interface {
        Create(ctx context.Context, wsConnection *model.WebsocketConnection) (err error)
    }

service.go: the request validates and binds itself; Execute takes a request and returns a response or an error. Services never
see `http.ResponseWriter`.

    type EndpointReq struct {
        model.WebsocketConnection
    }

    func (this *EndpointReq) Validate() error {
        if this.Host == "" {
            return errors.New("host is mandatory")
        }
        return nil
    }

    // update/service.go
    func (this *EndpointReq) BindPath(req *http.Request) {
        this.ID = model.WebsocketConnectionID(chi.URLParam(req, "websocket_connection_id"))
    }

    func (this *Endpoint) Execute(ctx context.Context, req *EndpointReq) (resp *model.WebsocketConnection, err error) {
        identityID, err := this.auth.GetIdentityID(ctx)
        if err != nil {
            return nil, st.StackTrace(this.auth.GetIdentityID, err)
        }

        wsConnection, err := this.db.WebsocketConnection.Find(ctx, req.ID, identityID)
        if err != nil {
            return nil, st.StackTrace(this.db.WebsocketConnection.Find, err)
        }
        if wsConnection == nil {
            return nil, http_outcome.NewError(http.StatusNotFound, errors.New("not found"))
        }
        ...
    }

Ownership is checked in the query (`where id = $1 and owner_id = $2`), not afterwards.

## Shared packages (small, under `libs/`)
`http_outcome`: the generic adapter. A handler is a function `func(ctx, *Req) (*Resp, error)`.

    type Endpoint[Req any, Resp any] func(ctx context.Context, req *Req) (*Resp, error)
    type NoRequest struct{}
    type NoResponse struct{} // answered with 204

    type PathBindable interface{ BindPath(r *http.Request) }
    type QueryBindable interface{ BindQuery(r *http.Request) }
    type Validator interface{ Validate() error }

    func AdaptEndpoint[Req any, Resp any](ep Endpoint[Req, Resp]) http.HandlerFunc
    //   decode JSON body (unless NoRequest) -> BindPath -> BindQuery -> Validate (400) -> ep(ctx, &req)
    //   *ErrorResponse -> its status; anything else -> 500; NoResponse -> 204; else JSON

    func NewError(statusCode int, err error) *ErrorResponse   // an error that carries its HTTP status

`stack_trace`: wrap every error as it travels up, naming the function that failed.

    return nil, st.StackTrace(this.db.WebsocketConnection.Create, err)   // "pkg.Caller -> pkg.Create: cause"

## Where we deliberately differ from ws2wh
- **Errors.** ws2wh writes `err.Error()` as a plain-text body, so a 500 leaks `pkg.Func -> pkg.Func: pq: ...` to the client and
  successes are JSON while errors are not. Here the adapter must answer `{error, code}` (the existing body), send a safe fixed
  message for 5xx, and log the wrapped detail on the server. Keep the existing sentinel errors; map them to `NewError` statuses
  in the service, not in the handler.
- **Update must write what it validates.** ws2wh's update validates every field and writes one. Write all the fields, and return the stored row.
- **Sessions** stay in their own table (ws2wh has one session slot per identity).
- **Account deletion** is one transaction.
- **Ids** stay int64 and **delete** stays a hard delete for now; no UUIDs or soft delete unless the user asks.
- **Receivers.** ws2wh uses `this`. Use it everywhere in the converted code, never a mix.

## Tests: ordered the way the code runs
Per endpoint: `TestNew`, `TestEndpointReq_Validate` (table), `TestEndpoint_Execute` (table). In Execute, one case per failure
point in the order of the code (auth error, db error, ..., success), then success. Mocks implement the endpoint's own interface
with a `...Func` field per method, and fail the test if an unexpected method is called.

    {
        Name: "db.WebsocketConnection.Create_error",
        Endpoint: &Endpoint{
            auth: &MiddlewareTestOrchestration{T: t, GetIdentityIDFunc: func(ctx context.Context) (model.IdentityID, error) { return "user123", nil }},
            db:   &DB{WebsocketConnection: &mockWebsocketConnection{T: t, CreateFunc: func(...) error { return errors.New("dbErr") }}},
        },
        Input:   &EndpointReq{},
        WantErr: true,
    },

Put the generic runners (`RunValidateTests`, `RunExecuteTests`, `RunBindPathTests`) and the mocks in `libs/mockprovider`.

## Hygiene (from ws2wh's `code_hygiene.md`)
- Interfaces are listed alphabetically.
- Tests are ordered in the way the code is executed.
- Few comments; say why, not what.
