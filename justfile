# lgln-citygml-proxy justfile
# Development automation for the LGLN CityGML proxy service

set shell := ["bash", "-uc"]

# Default recipe - show available commands
default:
    @just --list

# Install all missing development tools (formatters and linters)
setup:
    #!/usr/bin/env bash
    set -euo pipefail

    go_tool() {
        local bin=$1 pkg=$2
        if command -v "$bin" >/dev/null 2>&1; then
            echo "  $bin: already installed"
        else
            echo "  $bin: installing…"
            go install "$pkg"
        fi
    }

    echo "Go tools…"
    go_tool gofumpt       mvdan.cc/gofumpt@latest
    go_tool gci           github.com/daixiang0/gci@latest
    go_tool shfmt         mvdan.cc/sh/v3/cmd/shfmt@latest
    go_tool golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint@latest

    echo "prettier…"
    if command -v prettier >/dev/null 2>&1; then
        echo "  prettier: already installed"
    else
        echo "  prettier: installing via npm…"
        npm install -g prettier
    fi

    echo "treefmt…"
    if command -v treefmt >/dev/null 2>&1; then
        echo "  treefmt: already installed"
    else
        echo "  treefmt: downloading latest release…"
        OS=$(uname -s | tr '[:upper:]' '[:lower:]')
        ARCH=$(uname -m)
        INSTALL_DIR="${HOME}/.local/bin"
        mkdir -p "$INSTALL_DIR"
        ASSET_URL=$(curl -fsSL https://api.github.com/repos/numtide/treefmt/releases/latest \
            | grep -o '"browser_download_url": *"[^"]*"' \
            | grep -i "$OS" | grep -i "$ARCH" | grep '\.tar\.gz"' \
            | head -1 \
            | sed 's/"browser_download_url": *"\(.*\)"/\1/')
        if [ -z "$ASSET_URL" ]; then
            echo "  Could not find a treefmt release for ${OS}/${ARCH}."
            echo "  Install manually from https://github.com/numtide/treefmt/releases"
            exit 1
        fi
        curl -fsSL "$ASSET_URL" | tar -xz -C "$INSTALL_DIR" treefmt
        echo "  treefmt installed to ${INSTALL_DIR}/treefmt"
        echo "  Ensure ${INSTALL_DIR} is in your PATH."
    fi

    echo "All tools ready."

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
