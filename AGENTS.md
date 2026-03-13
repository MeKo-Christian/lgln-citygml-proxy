# AGENTS.md

This file provides guidance to AI agents (Claude Code, Codex etc.) when working with code in this repository.

## Commands

All development automation uses [just](https://github.com/casey/just):

```sh
just build          # Build the proxy binary → bin/lgln-citygml-proxy
just test           # Run all tests
just lint           # Run golangci-lint
just fmt            # Format all code with treefmt
just check          # Run all checks: format + lint + test + go mod tidy
just run            # Run proxy server (go run . serve)
just build-wasm     # Build WebAssembly for the web demo
just serve-web      # Serve the web demo on http://localhost:8000
just docker-build   # Build Docker image
```

To run a single test package:

```sh
go test -v ./internal/proxy/...
```

### Tool dependencies (install manually)

- `treefmt`, `gofumpt`, `gci`, `shfmt`: for formatting
- `golangci-lint`: for linting
- `prettier`: for YAML/JSON/Markdown formatting

## Architecture

The project has two independent Go programs and a browser demo:

### 1. Proxy server (`main.go` + `cmd/serve.go`)

An HTTP proxy that caches LGLN Niedersachsen CityGML LoD1 and LoD2 tiles on disk and re-serves them. Tiles are 1 km grid squares identified by EPSG:25832 easting/northing coordinates in km.

**Request flow:**

- `GET /lod2/{easting}/{northing}` → `internal/server` → `internal/proxy.Fetcher.Get()` → serves `.gml` file
- `GET /lod2?bbox=west,south,east,north` → `internal/server` → `internal/proxy.Fetcher.BBoxTileCoords()` → concurrent `GetMulti()` → ZIP archive
- `GET /lod1/{easting}/{northing}` → same as `/lod2` but via the LoD1 Fetcher (different S3 bucket and filename prefix)
- `GET /lod1?bbox=west,south,east,north` → same as `/lod2?bbox` but via the LoD1 Fetcher
- OGC API Features routes (`/conformance`, `/collections`, `/collections/buildings/items`) → `internal/ogcapi` → LoD2 Fetcher

**Key packages:**

- `internal/proxy` — `Fetcher` caches tiles to disk; `New` targets `lod2.s3.eu-de.cloud-object-storage.appdomain.cloud`, `NewLoD1` targets `lod1.s3.eu-de.cloud-object-storage.appdomain.cloud`; optional STAC integration for cache invalidation
- `internal/server` — registers HTTP routes, calls the Fetcher
- `internal/ogcapi` — OGC API Features-compliant handler; parses CityGML and converts to GeoJSON features
- `internal/stac` — STAC API client for tile discovery and freshness checks (queries `Aktualitaet` property)
- `internal/bbox` — EPSG:25832 bounding box parsing and tile coordinate enumeration
- `internal/utm` — UTM ↔ WGS84 conversion and building coordinate transformation

**STAC integration** is opt-in via `--stac-url`. Without it, `BBoxTileCoords` derives tile coordinates purely from the bbox grid arithmetic.

### 2. WebAssembly module (`cmd/wasm/main.go`)

Compiled to `web/citygml.wasm` for the browser demo. Exposes a single global JS function `parseCityGML(Uint8Array)` that returns a JSON object containing GeoJSON, a 3D scene graph, metadata, and validation findings. Uses `github.com/cwbudde/go-citygml` for parsing.

### 3. Web demo (`web/`)

Static HTML/JS app that loads the WASM, fetches Hannover tile GML files from `web/tiles/`, parses them client-side, and renders them in a Leaflet map (`modules/map.js`) and a Three.js 3D scene (`modules/scene.js`). Run `just fetch-tiles` then `just build-wasm` before serving.

## Key external dependency

`github.com/cwbudde/go-citygml` — CityGML parser library providing `citygml.Read()`, `citygml.Validate()`, `geojson.FromDocument()`, `geojson.BuildingFeature()`, and coordinate types. Used by both the server (ogcapi) and the WASM module.

## Tile naming convention

Tiles follow the pattern `{LoD}_32_{easting}_{northing}_1_ni.gml` where `{LoD}` is `LoD2` or `LoD1` and easting/northing are EPSG:25832 km grid coordinates (e.g., `LoD2_32_550_5800_1_ni.gml`, `LoD1_32_550_5800_1_ni.gml`).

## Deployment

Docker image runs `lgln-citygml-proxy serve --port 8080 --cache-dir /cache`. The cache directory should be mounted as a persistent volume. Helm chart is in `deployments/lgln-citygml-proxy/`.
