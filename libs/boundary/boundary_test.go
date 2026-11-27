package boundary

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// programs are the four deployable areas. Each one is the main package at the root of its services/<name> module, next to
// its routes, endpoints and the rest of the tree. The tools (admin, migrate, seed) are mains under tools/.
var programs = []string{"accounts", "community", "history", "puzzles"}

// The rules, in short (see STYLE.md, Deployables):
//  1. Nothing imports a program (services/<name>), a tool (tools/...) or the integration tests.
//  2. libs do not import services, except the two listed exceptions below.
//  3. A service imports another service only through its model, or, from its routes package, through its postgres
//     client (wiring). Endpoints, models, workers and clients never import another service.
//  4. Inside a service: models are leaves, and only routes import endpoints.
//  5. A program's main imports its own routes, endpoints, worker and session, and only the postgres clients and models
//     of the other services.
//
// To allow something new on purpose, change a rule here and say why in STYLE.md.
func check(graph map[string][]string) []string {
	var out []string

	for from, imports := range graph {
		for _, to := range imports {
			if reason := violation(from, to); reason != "" {
				out = append(out, fmt.Sprintf("%s imports %s: %s", from, to, reason))
			}
		}
	}

	sort.Strings(out)

	return out
}

func violation(from, to string) string {
	if _, ok := programName(to); ok || strings.HasPrefix(to, "tools/") || to == "integration" || strings.HasPrefix(to, "integration/") {
		return "nothing may import a program, a tool or the integration tests"
	}

	if _, ok := programName(from); ok {
		return programRule(from, to)
	}

	switch {
	case strings.HasPrefix(from, "libs/"):
		return libsRule(from, to)
	case strings.HasPrefix(from, "services/"):
		return serviceRule(from, to)
	}

	return ""
}

// split reads "services/<service>/<layer>/..." into its service and layer. Anything else gives empty strings.
func split(path string) (service, layer string) {
	parts := strings.Split(path, "/")
	if parts[0] != "services" || len(parts) < 3 {
		return "", ""
	}

	return parts[1], parts[2]
}

func libsRule(from, to string) string {
	service, layer := split(to)
	if service == "" {
		return ""
	}

	// Known exceptions: the shared server core checks sessions, and the mocks need the models.
	if from == "libs/server" && (to == "services/accounts/model" || to == "services/accounts/session") {
		return ""
	}
	if from == "libs/mockprovider" && layer == "model" {
		return ""
	}

	return "libs must not depend on services (allowed: libs/server -> accounts model and session, libs/mockprovider -> models)"
}

func serviceRule(from, to string) string {
	toService, toLayer := split(to)
	if toService == "" {
		return ""
	}
	fromService, fromLayer := split(from)

	if fromService != toService {
		if toLayer == "model" || (fromLayer == "routes" && toLayer == "postgres") {
			return ""
		}

		return "a service may import another service's model, or its postgres client from routes only"
	}

	if fromLayer == "model" && toLayer != "model" {
		return "models are leaves and must not import their own service's other layers"
	}
	if toLayer == "routes" || (toLayer == "endpoints" && to != from) {
		if fromLayer != "routes" {
			return "only routes import endpoints, and nothing imports routes"
		}
	}

	return ""
}

// programName says if path is the main package of a program (services/<name>, not a package below it).
func programName(path string) (string, bool) {
	name, ok := strings.CutPrefix(path, "services/")

	return name, ok && contains(programs, name)
}

// programRule is the rule for the main of a program. The tools under tools/ have none: they may reach where they need to.
func programRule(from, to string) string {
	name, _ := programName(from)
	service, layer := split(to)
	if service == "" || service == name {
		return ""
	}
	if layer == "postgres" || layer == "model" {
		return ""
	}

	return "a program may only use another service's postgres client or model (wiring)"
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}

	return false
}

// TestLayout checks the real import graph of the module.
func TestLayout(t *testing.T) {
	graph, _ := loadGraph(t)

	if len(graph) < 20 {
		t.Fatalf("only %d packages found; the graph did not load", len(graph))
	}

	// Guards against a loader that drops every edge, which would make the check below pass on nothing.
	if !contains(graph["services/community"], "services/community/routes") {
		t.Fatalf("known edge services/community -> services/community/routes is missing; got %v", graph["services/community"])
	}

	for _, v := range check(graph) {
		t.Error(v)
	}
}

// TestRules proves each rule fires and each allowed edge passes, so a broken rule cannot pass silently.
func TestRules(t *testing.T) {
	cases := []struct {
		name string
		from string
		to   string
		bad  bool
	}{
		{"endpoint -> own model", "services/puzzles/endpoints/publish", "services/puzzles/model", false},
		{"endpoint -> lib", "services/puzzles/endpoints/publish", "libs/http_outcome", false},
		{"routes -> own endpoint", "services/puzzles/routes", "services/puzzles/endpoints/publish", false},
		{"routes -> other postgres", "services/community/routes", "services/history/postgres", false},
		{"any -> other model", "services/community/endpoints/postcomment", "services/puzzles/model", false},
		{"model -> other model", "services/community/model", "services/puzzles/model", false},
		{"lib/server -> accounts session", "libs/server", "services/accounts/session", false},
		{"mockprovider -> model", "libs/mockprovider", "services/history/model", false},
		{"program -> own routes", "services/community", "services/community/routes", false},
		{"program -> other postgres", "services/community", "services/history/postgres", false},
		{"tool -> endpoints", "tools/seed", "services/puzzles/endpoints/publish", false},

		{"endpoint -> other service postgres", "services/community/endpoints/feed", "services/puzzles/postgres", true},
		{"endpoint -> other service endpoint", "services/community/endpoints/feed", "services/puzzles/endpoints/list", true},
		{"worker -> other service postgres", "services/puzzles/worker", "services/accounts/postgres", true},
		{"postgres -> other service postgres", "services/history/postgres", "services/puzzles/postgres", true},
		{"routes -> other routes", "services/history/routes", "services/puzzles/routes", true},
		{"endpoint -> sibling endpoint", "services/puzzles/endpoints/publish", "services/puzzles/endpoints/list", true},
		{"endpoint -> routes", "services/puzzles/endpoints/publish", "services/puzzles/routes", true},
		{"model -> own postgres", "services/puzzles/model", "services/puzzles/postgres", true},
		{"lib -> endpoint", "libs/http_outcome", "services/puzzles/endpoints/publish", true},
		{"lib/server -> history model", "libs/server", "services/history/model", true},
		{"service -> program", "services/puzzles/routes", "services/puzzles", true},
		{"lib -> integration", "libs/dbtest", "integration", true},
		{"service -> tool", "services/puzzles/routes", "tools/seed", true},
		{"program -> other routes", "services/accounts", "services/puzzles/routes", true},
		{"program -> other worker", "services/history", "services/puzzles/worker", true},
	}

	for _, c := range cases {
		got := check(map[string][]string{c.from: {c.to}})
		if c.bad && len(got) == 0 {
			t.Errorf("%s: %s -> %s should be rejected", c.name, c.from, c.to)
		}
		if !c.bad && len(got) != 0 {
			t.Errorf("%s: %s -> %s should be allowed, got %v", c.name, c.from, c.to, got)
		}
	}
}

// modulePrefix is the import path every package of the API starts with. Each module's path is this prefix plus its
// directory (libs/db is github.com/Sili-Sudoku/api/libs/db), so import paths do not depend on how modules are cut.
const modulePrefix = "github.com/Sili-Sudoku/api/"

// apiRoot is the directory with the workspace file that lists every module.
func apiRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	return root
}

// loadGraph asks the go tool for the imports of every package, using the workspace at the API root (the one go.work
// that lists all modules) so it sees them all, wherever the test runs from. prod has the imports of the code itself;
// all adds the imports of its tests.
func loadGraph(t *testing.T) (prod, all map[string][]string) {
	t.Helper()

	root := apiRoot(t)
	cmd := exec.Command("go", "list", "-f",
		`{{.ImportPath}}|{{join .Imports " "}}|{{join .TestImports " "}} {{join .XTestImports " "}}`, modulePrefix+"...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK="+filepath.Join(root, "go.work"))
	listing, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}

	prod, all = map[string][]string{}, map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(listing)), "\n") {
		parts := strings.Split(line, "|")
		pkg := strings.TrimPrefix(parts[0], modulePrefix)

		prod[pkg], all[pkg] = nil, nil
		for k, list := range parts[1:] {
			for _, imp := range strings.Fields(list) {
				if rel, ok := strings.CutPrefix(imp, modulePrefix); ok {
					if k == 0 {
						prod[pkg] = append(prod[pkg], rel)
					}

					all[pkg] = append(all[pkg], rel)
				}
			}
		}
	}

	return prod, all
}

// workUse reads the directories a go.work file lists in its use block, relative to the API root.
func workUse(t *testing.T, root, dir string) []string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(root, dir, "go.work"))
	if err != nil {
		t.Errorf("%s: %v", dir, err)

		return nil
	}

	var out []string
	inUse := false
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "use (":
			inUse = true
		case line == ")":
			inUse = false
		case inUse && line != "":
			rel, err := filepath.Rel(root, filepath.Join(root, dir, line))
			if err != nil {
				t.Fatal(err)
			}

			out = append(out, rel)
		}
	}

	sort.Strings(out)

	return out
}

// TestWorkspaces keeps the modules apart. Every module is cut by a go.mod (its outside dependencies) and a go.work (the
// sibling modules it builds with). The go.work must list exactly the modules the code imports, directly or through
// another module, tests included. A missing entry would not build; an extra one would quietly pull a dependency (a
// future Google login SDK, say) into services that never use it, so this fails on both.
// To add a module: create its go.mod and go.work, add it to the root go.work, and add it to the go.work of every module
// that imports it (and of the modules that import those). Run `go work sync` for its outside dependencies.
func TestWorkspaces(t *testing.T) {
	root := apiRoot(t)
	modules := workUse(t, root, ".")
	if len(modules) < 20 {
		t.Fatalf("only %d modules in the root go.work", len(modules))
	}

	moduleOf := func(pkg string) string {
		best := ""
		for _, m := range modules {
			if (pkg == m || strings.HasPrefix(pkg, m+"/")) && len(m) > len(best) {
				best = m
			}
		}

		return best
	}

	_, all := loadGraph(t)
	edges := map[string]map[string]bool{}
	for pkg, imports := range all {
		from := moduleOf(pkg)
		if from == "" {
			t.Errorf("package %s is in no module of the root go.work", pkg)

			continue
		}

		for _, imp := range imports {
			if to := moduleOf(imp); to != "" && to != from {
				if edges[from] == nil {
					edges[from] = map[string]bool{}
				}

				edges[from][to] = true
			}
		}
	}

	for _, m := range modules {
		want := map[string]bool{m: true}
		for todo := []string{m}; len(todo) > 0; {
			next := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			for to := range edges[next] {
				if !want[to] {
					want[to] = true
					todo = append(todo, to)
				}
			}
		}

		got := map[string]bool{}
		for _, u := range workUse(t, root, m) {
			got[u] = true
		}

		for u := range want {
			if !got[u] {
				t.Errorf("%s/go.work is missing %s, which its code imports", m, u)
			}
		}

		for u := range got {
			if !want[u] {
				t.Errorf("%s/go.work lists %s, which nothing in %s imports (directly or through another module)", m, u, m)
			}
		}

		mod, err := os.ReadFile(filepath.Join(root, m, "go.mod"))
		if err != nil {
			t.Errorf("%s: %v", m, err)

			continue
		}

		if strings.Contains(strings.SplitN(string(mod), "\n", 2)[1], modulePrefix) {
			t.Errorf("%s/go.mod requires a sibling module; siblings come from go.work, go.mod lists outside dependencies only", m)
		}
	}
}

// dockerPrograms are the programs that are built into images: each has a main.dockerfile next to its main package.
var dockerPrograms = []string{"services/accounts", "services/community", "services/history", "services/puzzles", "tools/migrate"}

// dockerfile is the main.dockerfile of the program in dir, made from its go.work. It copies the go.mod (and go.sum) of
// every module in the go.work, downloads, and only then copies the modules and builds, so the image holds the program's
// closure and nothing else, and the download layer stays cached. The Node image of puzzles also carries the validator.
func dockerfile(t *testing.T, root, dir string) string {
	t.Helper()

	mods := workUse(t, root, dir)
	var tops []string // the modules that no other listed module contains: copying one copies the ones inside it
	for _, m := range mods {
		nested := false
		for _, o := range mods {
			if o != m && strings.HasPrefix(m, o+"/") {
				nested = true
			}
		}

		if !nested {
			tops = append(tops, m)
		}
	}

	exists := func(path string) bool {
		_, err := os.Stat(filepath.Join(root, path))

		return err == nil
	}

	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	w("# Generated by `UPDATE_DOCKERFILES=1 go test github.com/Sili-Sudoku/api/libs/boundary -run Dockerfiles`. Do not edit.")
	w("# Builds the program in %s. Context: the api/ directory. Only the modules in its go.work are copied, so the image", dir)
	w("# cannot depend on anything else. Dependencies are downloaded before the code is copied, so they stay cached.")
	w("FROM golang:1.27 AS build")
	w("WORKDIR /opt/src/project")
	w("")
	for _, m := range mods {
		for _, f := range []string{"go.mod", "go.sum", "go.work.sum"} {
			path := m + "/" + f
			if exists(path) && (f != "go.work.sum" || m == dir) {
				w("COPY ./%s ./%s", path, path)
			}
		}
	}

	w("COPY ./%s/go.work ./%s/go.work", dir, dir)
	w("")
	w("WORKDIR /opt/src/project/%s", dir)
	w("RUN go mod download")
	w("")
	w("WORKDIR /opt/src/project")
	for _, m := range tops {
		w("COPY ./%s ./%s", m, m)
	}

	w("")
	w("WORKDIR /opt/src/project/%s", dir)
	w("RUN CGO_ENABLED=0 go build -o /out/app .")
	w("")
	if dir == "services/puzzles" {
		w("# Node 24 plus the bundled validate-puzzle tool (a single file; it needs nothing else). The release script puts it in")
		w("# dist-tools/ first (in web/: npm run build:validator, then copy web/dist-tools/validate-puzzle here). Without it the")
		w("# program would accept every puzzle, so REQUIRE_VALIDATOR=true makes it refuse to start.")
		w("FROM node:24-slim")
		w("COPY --from=build /out/app /app")
		w("COPY dist-tools/validate-puzzle /usr/local/bin/validate-puzzle")
		w("# The slim image has no CA certificates, and Go needs them for HTTPS (the SudokuPad import fetches sudokupad.app).")
		w("COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt")
		w("ENV VALIDATE_CMD=/usr/local/bin/validate-puzzle")
		w("USER node")
	} else {
		w("FROM gcr.io/distroless/static-debian12")
		w("COPY --from=build /out/app /app")
		w("USER nonroot")
	}

	w(`ENTRYPOINT ["/app"]`)

	return b.String()
}

// TestDockerfiles keeps each program's main.dockerfile equal to what dockerfile makes of its go.work: the go.mod and
// go.sum of exactly the modules it lists. A missing module would not build; an extra one would carry a dependency the
// program does not use; a missing go.sum would fail the download. After a change of imports, `go.work` files or go.sum
// files, run it with UPDATE_DOCKERFILES=1 to write the files, and review the diff.
func TestDockerfiles(t *testing.T) {
	root := apiRoot(t)
	for _, dir := range dockerPrograms {
		want := dockerfile(t, root, dir)
		path := filepath.Join(root, dir, "main.dockerfile")
		if os.Getenv("UPDATE_DOCKERFILES") != "" {
			if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
				t.Fatal(err)
			}

			continue
		}

		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("%s/main.dockerfile is out of date with its go.work: run UPDATE_DOCKERFILES=1 go test github.com/Sili-Sudoku/api/libs/boundary -run Dockerfiles", dir)
		}
	}
}
