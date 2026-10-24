default:
    @just --list

# Build local binary
build:
    @go build -o bin/ctfhelper cmd/ctfhelper/main.go

# Build optimized production binary for Linux (Docker)
build-prod:
    @echo "Building production binary for Linux (optimized for Docker)..."
    @env CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags "libsqlite3 linux" -trimpath -ldflags="-s -w" -o bin/ctfhelper cmd/ctfhelper/main.go

# Run the application (e.g. `just run "--flag"`)
run ARGS="": build
    @./bin/ctfhelper {{ ARGS }}

# Run golangci-lint
lint:
    @go work sync
    @go list -f '{{ "{{" }}.Dir}}' -m | xargs golangci-lint run --fix

# Format code with gofumpt
fmt:
    @go work sync
    @go list -f '{{ "{{" }}.Dir}}' -m | xargs gofumpt -w -d
