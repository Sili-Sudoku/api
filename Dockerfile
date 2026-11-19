# One image per program: docker build --build-arg SERVICE=accounts|puzzles|community|history|migrate .
# Not built in the claude-box (nested podman cannot build); build and try it on the host.
# BASE must be declared before the first FROM to be usable in a later FROM.
ARG BASE=gcr.io/distroless/static-debian12

# Each module has its own go.work, so building inside cmd/<program> sees only the modules that program needs.
FROM golang:1.27 AS build
ARG SERVICE
WORKDIR /src
COPY . .
WORKDIR /src/cmd/${SERVICE}
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/app .

# The puzzles image also needs the validate-puzzle tool from the web repo and Node 24 to run it (VALIDATE_CMD).
# Build it with --build-arg BASE=node:24-slim and copy the tool in; the other programs need nothing.
FROM ${BASE}
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
