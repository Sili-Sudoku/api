package integration

import (
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	accountsroutes "github.com/Sili-Sudoku/api/services/accounts/routes"
	communityroutes "github.com/Sili-Sudoku/api/services/community/routes"
	historyroutes "github.com/Sili-Sudoku/api/services/history/routes"
	puzzlesroutes "github.com/Sili-Sudoku/api/services/puzzles/routes"
)

// programOf is the routing table of "Deployables" in STYLE.md: which program serves a route of the contract. The proxy in
// front of the programs (deploy/Caddyfile, the cloud load balancer) must send each route to the program named here.
var programOf = map[string]string{
	"/v1/accounts":              "accounts",
	"/v1/sessions":              "accounts",
	"/v1/sessions/current":      "accounts",
	"/v1/me":                    "accounts",
	"/v1/me/history":            "history",
	"/v1/me/history/{puzzleId}": "history",
	"/v1/me/following":          "community",
	"/v1/me/feed":               "community",
	"/v1/users/{name}":          "community",
	"/v1/users/{name}/follow":   "community",
	"/v1/comments/{id}":         "community",
	"/v1/comments/{id}/report":  "community",
	"/v1/puzzles/{id}/ratings":  "community",
	"/v1/puzzles/{id}/comments": "community",
	"/v1/puzzles/{id}/stats":    "community",
	"/v1/puzzles":               "puzzles",
	"/v1/puzzles/{id}":          "puzzles",
	"/v1/puzzles/{id}/owner":    "puzzles",
	"/v1/puzzles/{id}/report":   "puzzles",
}

// contractRoutes reads the method and path pairs of contract/openapi.yaml (health check excluded).
func contractRoutes(t *testing.T) (routes [][2]string) {
	t.Helper()

	raw, err := os.ReadFile("../contract/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	pathLine := regexp.MustCompile(`^  "(/v1/[^"]*)":$`)
	methodLine := regexp.MustCompile(`^    (get|put|post|patch|delete):$`)
	path := ""

	for _, line := range strings.Split(string(raw), "\n") {
		if m := pathLine.FindStringSubmatch(line); m != nil {
			path = m[1]
		} else if m := methodLine.FindStringSubmatch(line); m != nil && path != "" {
			routes = append(routes, [2]string{strings.ToUpper(m[1]), path})
		}
	}

	return routes
}

// notRouted reports whether the mux itself had no route (as opposed to an endpoint answering 404 for a missing row).
func notRouted(code int, body string) bool {
	return code == http.StatusMethodNotAllowed || strings.HasPrefix(body, "404 page not found")
}

// Every route of the contract is served by exactly one program, and it is the one the routing table names.
func TestEveryContractRouteBelongsToOneProgram(t *testing.T) {
	e := newEnv(t)
	programs := map[string]http.Handler{
		"accounts":  accountsroutes.New(e.accounts),
		"community": communityroutes.New(e.community),
		"history":   historyroutes.New(e.history),
		"puzzles":   puzzlesroutes.New(e.puzzles),
	}

	routes := contractRoutes(t)
	if len(routes) < 25 {
		t.Fatalf("read only %d routes from the contract: the parser is out of step with the file", len(routes))
	}

	for _, rt := range routes {
		method, path := rt[0], rt[1]
		want, ok := programOf[path]
		if !ok {
			t.Errorf("%s %s is in the contract but not in the routing table", method, path)
			continue
		}

		concrete := regexp.MustCompile(`\{[^}]*\}`).ReplaceAllString(path, "x1")

		var served []string
		for name, h := range programs {
			w := do(h, method, concrete, "")
			if !notRouted(w.Code, w.Body.String()) {
				served = append(served, name)
			}
		}

		if len(served) != 1 || served[0] != want {
			t.Errorf("%s %s: served by %v, want only %s", method, path, served, want)
		}
	}
}

// The proxy rules route every contract path to the program of the routing table. Caddy's path_regexp is a Go regexp
// matched against the request path, so this test applies the real patterns from deploy/Caddyfile in the real order.
func TestCaddyfileRoutesTheContract(t *testing.T) {
	raw, err := os.ReadFile("../deploy/Caddyfile")
	if err != nil {
		t.Fatal(err)
	}

	matchers := map[string]*regexp.Regexp{}
	declared := regexp.MustCompile(`^\s*@(\w+) path_regexp (\S+)$`)
	proxied := regexp.MustCompile(`^\s*reverse_proxy @(\w+) `)

	var order []string

	for _, line := range strings.Split(string(raw), "\n") {
		if m := declared.FindStringSubmatch(line); m != nil {
			matchers[m[1]] = regexp.MustCompile(m[2])
		} else if m := proxied.FindStringSubmatch(line); m != nil {
			order = append(order, m[1])
		}
	}

	if len(order) != 4 {
		t.Fatalf("expected 4 proxied programs in the Caddyfile, found %v", order)
	}

	for _, rt := range contractRoutes(t) {
		path := rt[1]
		concrete := regexp.MustCompile(`\{[^}]*\}`).ReplaceAllString(path, "x1")

		got := "none"

		for _, name := range order {
			if matchers[name].MatchString(concrete) {
				got = name
				break
			}
		}

		if got != programOf[path] {
			t.Errorf("%s: the Caddyfile sends it to %s, want %s", path, got, programOf[path])
		}
	}
}

// The release tool builds the gateway's routes from deploy/deploy.toml: each [[service]] has a name and a list of path
// patterns (TOML single-quoted strings, so they are plain Go regexps), and the first service that matches wins. This
// reads that file the way the gateway uses it and checks every contract path reaches the program of the routing table.
func TestDeployManifestRoutesTheContract(t *testing.T) {
	raw, err := os.ReadFile("../deploy/deploy.toml")
	if err != nil {
		t.Fatal(err)
	}

	type service struct {
		name     string
		patterns []*regexp.Regexp
	}

	var services []service

	nameLine := regexp.MustCompile(`^\s*name\s*=\s*"([^"]+)"`)
	quoted := regexp.MustCompile(`'([^']*)'`)
	inService, inRoutes := false, false

	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "[[service]]":
			services = append(services, service{})
			inService, inRoutes = true, false
		case strings.HasPrefix(trimmed, "["):
			inService, inRoutes = false, false
		case inService && nameLine.MatchString(line):
			services[len(services)-1].name = nameLine.FindStringSubmatch(line)[1]
		case inService && strings.HasPrefix(trimmed, "routes"):
			inRoutes = true

			fallthrough
		case inService && inRoutes:
			for _, m := range quoted.FindAllStringSubmatch(line, -1) {
				s := &services[len(services)-1]
				s.patterns = append(s.patterns, regexp.MustCompile(m[1]))
			}

			if strings.Contains(line, "]") {
				inRoutes = false
			}
		}
	}

	if len(services) != 4 {
		t.Fatalf("expected 4 services in deploy.toml, found %d", len(services))
	}

	for _, s := range services {
		if s.name == "" || len(s.patterns) == 0 {
			t.Fatalf("service %q has no name or no routes", s.name)
		}
	}

	for _, rt := range contractRoutes(t) {
		path := rt[1]
		concrete := regexp.MustCompile(`\{[^}]*\}`).ReplaceAllString(path, "x1")

		got := "none"

	find:
		for _, s := range services {
			for _, p := range s.patterns {
				if p.MatchString(concrete) {
					got = s.name
					break find
				}
			}
		}

		if got != programOf[path] {
			t.Errorf("%s: deploy.toml sends it to %s, want %s", path, got, programOf[path])
		}
	}
}
