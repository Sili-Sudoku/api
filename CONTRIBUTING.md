# Contributing

Small pull requests are welcome. For a new feature, open an issue first. Then we can check that it fits the project.

Before you open a pull request, run this command in the repository directory:

    gofmt -l . && go vet github.com/Sili-Sudoku/api/... && go test -race github.com/Sili-Sudoku/api/...

The command must print no file names from `gofmt`, and all tests must pass.

The database tests need a database. Set `TEST_DATABASE_URL` to use them. Without it, the tests skip. Use a database that
holds no data that you want to keep. See the [README](README.md) for the details.

The code rules are in [`STYLE.md`](STYLE.md) and [`CONVENTIONS.md`](CONVENTIONS.md). If you change a `go.work` file, run
`UPDATE_DOCKERFILES=1 go test github.com/Sili-Sudoku/api/libs/boundary -run Dockerfiles`. This command rewrites the
generated `main.dockerfile` files.

The project uses the [Business Source License](LICENSE). Your contribution goes under the same license.
