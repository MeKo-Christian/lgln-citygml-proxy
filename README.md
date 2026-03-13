# lgln-citygml-proxy

A caching HTTP proxy for the [LGLN Niedersachsen](https://www.lgln.niedersachsen.de/) CityGML LoD2 3D building dataset, with an OGC API Features endpoint and a browser-based viewer.

## What it does

LGLN publishes CityGML LoD2 tiles (1 km × 1 km grid, EPSG:25832) as individual `.gml` files on an S3-compatible store. This proxy:

- Fetches tiles on demand and caches them to disk
- Exposes a simple tile API and a bounding-box bulk-download endpoint
- Exposes an [OGC API – Features](https://ogcapi.ogc.org/features/) endpoint that converts tiles to GeoJSON on the fly
- Optionally queries the [LGLN STAC API](https://lod.stac.lgln.niedersachsen.de) for tile discovery and cache freshness

## API

| Endpoint | Description |
|---|---|
| `GET /lod2/{easting}/{northing}` | Single tile as `application/gml+xml` |
| `GET /lod2?bbox=west,south,east,north` | All tiles in bbox as a ZIP archive (max 100 tiles, EPSG:25832) |
| `GET /collections/buildings/items?bbox=...` | Buildings as GeoJSON (OGC API Features) |
| `GET /collections` | OGC API collection listing |
| `GET /conformance` | OGC API conformance classes |
| `GET /health` | Health check |

Tile coordinates are EPSG:25832 easting/northing in **kilometres** (e.g., `/lod2/550/5800` for Hannover city centre).

## Running

### Docker

```sh
docker run --rm -p 8080:8080 -v ./cache:/cache \
  ghcr.io/meko-tech/lgln-citygml-proxy
```

With STAC integration:

```sh
docker run --rm -p 8080:8080 -v ./cache:/cache \
  ghcr.io/meko-tech/lgln-citygml-proxy \
  serve --stac-url https://lod.stac.lgln.niedersachsen.de
```

### Binary

```sh
go install github.com/meko-tech/lgln-citygml-proxy@latest
lgln-citygml-proxy serve --port 8080 --cache-dir ./cache
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `--port` / `-p` | `8080` | Port to listen on |
| `--cache-dir` / `-c` | `./cache` | Directory for cached tile files |
| `--stac-url` | _(disabled)_ | STAC API base URL for tile discovery and cache freshness |

## Kubernetes / Helm

```sh
helm upgrade --install lgln-citygml-proxy ./deployments/lgln-citygml-proxy \
  --set persistence.enabled=true \
  --set ingress.enabled=true
```

The chart image defaults to `ghcr.io/meko-tech/lgln-citygml-proxy`. Enable `persistence` to retain the tile cache across pod restarts (defaults to 10 Gi).

## Browser demo

A static web demo visualises Hannover city-centre tiles in a 2D map (Leaflet) and a 3D scene (Three.js). CityGML parsing runs entirely in the browser via a Go WebAssembly module.

The demo is automatically deployed to GitHub Pages on every push to `main`.

To run it locally:

```sh
just build-wasm   # compile cmd/wasm → web/citygml.wasm
just fetch-tiles  # download 4 Hannover tiles into web/tiles/
just serve-web    # http://localhost:8000
```

## Development

Requires [just](https://github.com/casey/just) and Go 1.25+.

```sh
just build    # build the proxy binary
just test     # run all tests
just lint     # run golangci-lint
just fmt      # format all code
just check    # format + lint + test + go mod tidy
```

Additional tools needed for formatting: `gofumpt`, `gci`, `shfmt`, `prettier`.

## License

Data from LGLN Niedersachsen is published under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).
