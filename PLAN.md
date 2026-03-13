# LGLN CityGML Proxy — Plan

A proxy server that provides API access to LGLN Niedersachsen's CityGML LoD2 3D building data.

## Data Source

LGLN publishes LoD2 CityGML tiles on IBM Cloud Object Storage:

```plain
https://lod2.s3.eu-de.cloud-object-storage.appdomain.cloud/LoD2_32_{easting_km}_{northing_km}_1_ni.gml
```

- **CRS:** EPSG:25832 (ETRS89/UTM Zone 32N)
- **Tile size:** 1×1 km
- **Format:** CityGML (.gml)
- **License:** CC BY 4.0 (LGLN Niedersachsen)
- **STAC API:** https://lod.stac.lgln.niedersachsen.de/collections/lod2
- **Tile index:** https://arcgis-geojson.s3.eu-de.cloud-object-storage.appdomain.cloud/lod2/lgln-opengeodata-lod2.geojson

## Phase 1 — Tile Proxy with Caching ✅

Single-tile proxy with local disk cache.

- `GET /lod2/{easting_km}/{northing_km}` — fetch a single CityGML tile
- Local disk cache (configurable directory)
- Transparent proxy: returns raw `.gml` content
- CLI: `serve --port 8080 --cache-dir ./cache`

## Phase 2 — Bounding Box Endpoint ✅

Fetch multiple tiles covering a geographic bounding box.

- `GET /lod2?bbox={west},{south},{east},{north}` — returns all tiles intersecting the bbox
- Coordinates in EPSG:25832 (meters) → converted to km tile grid
- Response: multipart or ZIP archive containing all matching tiles
- Parallel tile fetching with concurrency limit

## Phase 3 — OGC API Features ✅

Standards-compliant OGC API Features endpoint (like NRW's ogc-api.nrw.de).

- `GET /collections` — list available collections
- `GET /collections/buildings/items?bbox=...&limit=...` — query buildings
- CityGML → GeoJSON conversion for feature responses
- Pagination support
- Conformance classes: Core, GeoJSON, OGC API Features Part 1

## Phase 4 — STAC Integration

Use the LGLN STAC API for tile discovery instead of hardcoded URL patterns.

- Query `https://lod.stac.lgln.niedersachsen.de/collections/lod2/items` for spatial search
- Use STAC item metadata for freshness checks (Aktualitaet field)
- Cache invalidation based on STAC timestamps

## Phase 5 — LoD1 Support

Add LoD1 tiles alongside LoD2.

- `GET /lod1/{easting_km}/{northing_km}`
- `GET /lod1?bbox=...`
- Discover LoD1 URL pattern (likely similar scheme)
