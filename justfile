default:
    @just --list

# Build local binary
build:
    @go build -o bin/ctfhelper ./cmd/ctfhelper

# Build optimized production binary for Linux (Docker)
build-prod:
    @echo "Building production binary for Linux (optimized for Docker)..."
    @env CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags "libsqlite3 linux" -trimpath -ldflags="-s -w" -o bin/ctfhelper ./cmd/ctfhelper

# Run the application (e.g. `just run "--flag"`)
run ARGS="": build
    @./bin/ctfhelper {{ ARGS }}

# Run golangci-lint
lint:
    @golangci-lint run --fix

# Format code with gofumpt
fmt:
    @gofumpt -w -d .

# Run the tests
test:
    @go test -race ./...
