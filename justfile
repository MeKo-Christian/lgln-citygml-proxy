# lgln-citygml-proxy justfile
# Development automation for the LGLN CityGML proxy service

set shell := ["bash", "-uc"]

# Default recipe - show available commands
default:
    @just --list

# Note: Install dependencies manually
# treefmt: Download from https://github.com/numtide/treefmt/releases
# Go tools: go install mvdan.cc/gofumpt@latest && go install github.com/daixiang0/gci@latest
# golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
# prettier: npm install -g prettier

# Format all code using treefmt
fmt:
    treefmt --allow-missing-formatter

# Check if code is formatted correctly
check-formatted:
    treefmt --allow-missing-formatter --fail-on-change

# Run linters
lint:
    golangci-lint run --timeout=2m

# Run linters with auto-fix
lint-fix:
    golangci-lint run --fix --timeout=2m

# Ensure go.mod is tidy
check-tidy:
    go mod tidy
    git diff --exit-code go.mod go.sum

# Run all tests
test:
    go test -v -timeout 120s ./...

# Run tests with coverage
test-coverage:
    go test -v -timeout 120s -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

# Run all checks (formatting, linting, tests, tidiness)
check: check-formatted lint test check-tidy

# Build the proxy binary
build:
    go build -o bin/lgln-citygml-proxy .

# Build WebAssembly for the web demo
build-wasm:
    #!/usr/bin/env bash
    set -euo pipefail
    GOOS=js GOARCH=wasm go build -o web/citygml.wasm ./cmd/wasm
    GOROOT=$(go env GOROOT)
    if [ -f "$GOROOT/misc/wasm/wasm_exec.js" ]; then cp "$GOROOT/misc/wasm/wasm_exec.js" web/
    elif [ -f "$GOROOT/lib/wasm/wasm_exec.js" ]; then cp "$GOROOT/lib/wasm/wasm_exec.js" web/
    else echo "wasm_exec.js not found in GOROOT" >&2; exit 1; fi

# Download Hannover tiles for local web demo development
fetch-tiles:
    mkdir -p web/tiles
    curl -fsSL "https://lod2.s3.eu-de.cloud-object-storage.appdomain.cloud/LoD2_32_550_5800_1_ni.gml" -o web/tiles/LoD2_32_550_5800_1_ni.gml
    curl -fsSL "https://lod2.s3.eu-de.cloud-object-storage.appdomain.cloud/LoD2_32_550_5801_1_ni.gml" -o web/tiles/LoD2_32_550_5801_1_ni.gml
    curl -fsSL "https://lod2.s3.eu-de.cloud-object-storage.appdomain.cloud/LoD2_32_551_5800_1_ni.gml" -o web/tiles/LoD2_32_551_5800_1_ni.gml
    curl -fsSL "https://lod2.s3.eu-de.cloud-object-storage.appdomain.cloud/LoD2_32_551_5801_1_ni.gml" -o web/tiles/LoD2_32_551_5801_1_ni.gml

# Serve the web demo locally (requires just build-wasm && just fetch-tiles first)
serve-web:
    python3 -m http.server 8000 --directory web

# Install to $GOPATH/bin
install:
    go install .

# Clean build artifacts
clean:
    rm -rf bin/
    rm -f coverage.out coverage.html
    rm -f web/citygml.wasm web/wasm_exec.js

# Run the proxy server
run *ARGS:
    go run . serve {{ ARGS }}

# Build Docker image
docker-build TAG="lgln-citygml-proxy:latest":
    docker build -t {{ TAG }} .

# Run in Docker (mounts ./cache, exposes port 8080)
docker-run TAG="lgln-citygml-proxy:latest":
    docker run --rm -p 8080:8080 -v "$(pwd)/cache:/cache" {{ TAG }}
