# Phase 3 — OGC API Features Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add standards-compliant OGC API Features endpoints that serve CityGML building data as GeoJSON FeatureCollections with pagination.

**Architecture:**
1. `internal/utm/` — Pure-math UTM Zone 32N (ETRS89/EPSG:25832) → WGS84 coordinate transformation; also contains a helper to transform all geometry in a `types.Building` so the existing `go-citygml` GeoJSON converter can be reused.
2. `internal/ogcapi/` — OGC API Features HTTP handlers: conformance, collections list, single collection, and paginated items.
3. `internal/server/server.go` — Register the four new routes.

**Tech Stack:**
- `github.com/cwbudde/go-citygml` — already in `go.mod` (indirect); promoted to direct. Provides `citygml.Read()`, `types.Building/Document`, `geojson.BuildingFeature()`.
- Standard library only (`encoding/json`, `math`, `net/http`, `bytes`).

---

## Background: Key Types from go-citygml

```go
// citygml.Read(r io.Reader, opts citygml.Options) (*types.Document, error)
// types.Document{ Buildings []types.Building, CRS types.CRS, ... }
// types.Building{ ID string, Footprint *types.Polygon, MultiSurface *types.MultiSurface, Solid *types.Solid, ... }
// types.Point{ X, Y, Z float64 }  —  X=easting(m), Y=northing(m) in EPSG:25832
// geojson.BuildingFeature(b *types.Building) geojson.Feature
```

Coordinates in LGLN tiles are EPSG:25832 (UTM Zone 32N, ETRS89). WGS84 (lon/lat) is required for OGC API Features GeoJSON output.

---

## Task 1: UTM32N → WGS84 Coordinate Transformation

**Files:**
- Create: `internal/utm/utm.go`
- Create: `internal/utm/utm_test.go`

### Step 1: Write the failing test

```go
// internal/utm/utm_test.go
package utm_test

import (
	"math"
	"testing"

	"github.com/meko-tech/lgln-citygml-proxy/internal/utm"
)

func TestToWGS84_Hannover(t *testing.T) {
	// Known point near Hannover city centre
	// Easting 550000, Northing 5800000 → approximately lon=9.734°, lat=52.348°
	lon, lat := utm.ToWGS84(550000, 5800000)

	if math.Abs(lon-9.734) > 0.01 {
		t.Errorf("lon = %.4f, want ~9.734", lon)
	}
	if math.Abs(lat-52.348) > 0.01 {
		t.Errorf("lat = %.4f, want ~52.348", lat)
	}
}

func TestToWGS84_Origin(t *testing.T) {
	// The central meridian of Zone 32 is 9°E.
	// A point at E=500000, N=0 is on the equator at lon=9°.
	lon, lat := utm.ToWGS84(500000, 0)
	if math.Abs(lon-9.0) > 0.0001 {
		t.Errorf("lon = %.6f, want 9.0", lon)
	}
	if math.Abs(lat) > 0.0001 {
		t.Errorf("lat = %.6f, want 0.0", lat)
	}
}
```

### Step 2: Run test to verify it fails

```bash
cd /mnt/projekte/Code/lgln-citygml-proxy
go test ./internal/utm/... 2>&1
```
Expected: `cannot find package` or `undefined: utm.ToWGS84`

### Step 3: Implement minimal code

```go
// internal/utm/utm.go
// Package utm provides coordinate transformation from UTM Zone 32N (EPSG:25832) to WGS84.
package utm

import "math"

// WGS84 ellipsoid parameters.
const (
	a  = 6378137.0          // semi-major axis (m)
	fi = 298.257223563      // inverse flattening
	f  = 1 / fi
	b  = a * (1 - f)
	e2 = 2*f - f*f          // first eccentricity squared
	ep2 = e2 / (1 - e2)     // second eccentricity squared
)

// UTM Zone 32N (EPSG:25832) parameters.
const (
	k0      = 0.9996
	falseE  = 500000.0
	falseN  = 0.0           // northern hemisphere
	lambda0 = 9.0 * math.Pi / 180.0 // central meridian
)

// ToWGS84 converts UTM Zone 32N easting/northing (meters) to WGS84 longitude/latitude (degrees).
// Uses Snyder's Transverse Mercator inverse formulas (Map Projections – A Working Manual, p.60-61).
func ToWGS84(easting, northing float64) (lon, lat float64) {
	x := easting - falseE
	y := northing - falseN

	// Meridional arc distance divided by k0.
	M := y / k0

	// μ — the geodetic latitude for the foot of the perpendicular.
	aMu := a * (1 - e2/4 - 3*e2*e2/64 - 5*e2*e2*e2/256)
	mu := M / aMu

	e1 := (1 - math.Sqrt(1-e2)) / (1 + math.Sqrt(1-e2))

	phi1 := mu +
		(3*e1/2-27*e1*e1*e1/32)*math.Sin(2*mu) +
		(21*e1*e1/16-55*e1*e1*e1*e1/32)*math.Sin(4*mu) +
		(151*e1*e1*e1/96)*math.Sin(6*mu) +
		(1097*e1*e1*e1*e1/512)*math.Sin(8*mu)

	sinPhi1 := math.Sin(phi1)
	cosPhi1 := math.Cos(phi1)
	tanPhi1 := math.Tan(phi1)

	N1 := a / math.Sqrt(1-e2*sinPhi1*sinPhi1)
	T1 := tanPhi1 * tanPhi1
	C1 := ep2 * cosPhi1 * cosPhi1
	R1 := a * (1 - e2) / math.Pow(1-e2*sinPhi1*sinPhi1, 1.5)
	D := x / (N1 * k0)

	latRad := phi1 - (N1*tanPhi1/R1)*(D*D/2-
		(5+3*T1+10*C1-4*C1*C1-9*ep2)*D*D*D*D/24+
		(61+90*T1+298*C1+45*T1*T1-252*ep2-3*C1*C1)*D*D*D*D*D*D/720)

	lonRad := lambda0 + (D-
		(1+2*T1+C1)*D*D*D/6+
		(5-2*C1+28*T1-3*C1*C1+8*ep2+24*T1*T1)*D*D*D*D*D/120)/cosPhi1

	return lonRad * 180 / math.Pi, latRad * 180 / math.Pi
}
```

### Step 4: Run tests

```bash
go test ./internal/utm/... -v
```
Expected: both tests PASS.

### Step 5: Commit

```bash
git add internal/utm/utm.go internal/utm/utm_test.go
git commit -m "feat: add UTM Zone 32N to WGS84 coordinate transformation"
```

---

## Task 2: Building Geometry Transformer

**Files:**
- Modify: `internal/utm/utm.go` (add `TransformBuilding`)
- Modify: `internal/utm/utm_test.go` (add test)

This helper walks all geometry in a `types.Building` and applies `ToWGS84` to every point, returning a transformed copy usable by `geojson.BuildingFeature()`.

### Step 1: Add failing test

```go
// in utm_test.go
import "github.com/cwbudde/go-citygml/types"

func TestTransformBuilding_FootprintCoords(t *testing.T) {
	b := types.Building{
		ID: "test-building",
		Footprint: &types.Polygon{
			Exterior: types.Ring{
				Points: []types.Point{
					{X: 550000, Y: 5800000, Z: 50},
					{X: 550100, Y: 5800000, Z: 50},
					{X: 550100, Y: 5800100, Z: 50},
					{X: 550000, Y: 5800000, Z: 50},
				},
			},
		},
	}

	out := utm.TransformBuilding(b)
	pts := out.Footprint.Exterior.Points

	// After transform, X is longitude (~9.6°), Y is latitude (~52.3°)
	if pts[0].X > 20 || pts[0].X < 0 {
		t.Errorf("expected longitude ~9.6, got %.4f", pts[0].X)
	}
	if pts[0].Y > 60 || pts[0].Y < 40 {
		t.Errorf("expected latitude ~52.3, got %.4f", pts[0].Y)
	}
}
```

### Step 2: Run test — expect FAIL

```bash
go test ./internal/utm/... -v
```

### Step 3: Implement TransformBuilding

```go
// Add to internal/utm/utm.go

import "github.com/cwbudde/go-citygml/types"

// TransformBuilding returns a copy of b with all XY coordinates converted
// from UTM Zone 32N (EPSG:25832) to WGS84 longitude/latitude (degrees).
// Z (elevation) is preserved unchanged.
func TransformBuilding(b types.Building) types.Building {
	out := b
	if b.Footprint != nil {
		fp := transformPolygon(*b.Footprint)
		out.Footprint = &fp
	}
	if b.MultiSurface != nil {
		ms := transformMultiSurface(*b.MultiSurface)
		out.MultiSurface = &ms
	}
	if b.Solid != nil {
		out.Solid = &types.Solid{Exterior: transformMultiSurface(b.Solid.Exterior)}
	}
	// Transform BoundedBy surfaces.
	if len(b.BoundedBy) > 0 {
		out.BoundedBy = make([]types.Surface, len(b.BoundedBy))
		for i, s := range b.BoundedBy {
			out.BoundedBy[i] = types.Surface{
				ID:       s.ID,
				Type:     s.Type,
				Geometry: transformMultiSurface(s.Geometry),
			}
		}
	}
	return out
}

func transformPoint(p types.Point) types.Point {
	lon, lat := ToWGS84(p.X, p.Y)
	return types.Point{X: lon, Y: lat, Z: p.Z}
}

func transformRing(r types.Ring) types.Ring {
	pts := make([]types.Point, len(r.Points))
	for i, p := range r.Points {
		pts[i] = transformPoint(p)
	}
	return types.Ring{Points: pts}
}

func transformPolygon(p types.Polygon) types.Polygon {
	out := types.Polygon{Exterior: transformRing(p.Exterior)}
	for _, inner := range p.Interior {
		out.Interior = append(out.Interior, transformRing(inner))
	}
	return out
}

func transformMultiSurface(ms types.MultiSurface) types.MultiSurface {
	polys := make([]types.Polygon, len(ms.Polygons))
	for i, p := range ms.Polygons {
		polys[i] = transformPolygon(p)
	}
	return types.MultiSurface{Polygons: polys}
}
```

### Step 4: Run tests

```bash
go test ./internal/utm/... -v
```
Expected: all PASS.

### Step 5: Promote go-citygml to direct dependency

```bash
cd /mnt/projekte/Code/lgln-citygml-proxy
go get github.com/cwbudde/go-citygml@v0.0.0-20260311135839-345d9d02c1eb
```
This updates `go.mod` to remove the `// indirect` comment.

### Step 6: Commit

```bash
git add internal/utm/utm.go internal/utm/utm_test.go go.mod go.sum
git commit -m "feat: add building geometry transformer UTM32N→WGS84"
```

---

## Task 3: OGC API Response Types + Conformance Endpoint

**Files:**
- Create: `internal/ogcapi/handler.go`
- Create: `internal/ogcapi/handler_test.go`

### OGC API response types

The `geojson.FeatureCollection` from `go-citygml` doesn't have OGC API fields. We define our own extended types inside the `ogcapi` package.

```go
// internal/ogcapi/handler.go
package ogcapi

// Link is a hyperlink as defined by OGC API Features.
type Link struct {
	Href  string `json:"href"`
	Rel   string `json:"rel"`
	Type  string `json:"type,omitempty"`
	Title string `json:"title,omitempty"`
}

// Collection describes a single OGC API Features collection.
type Collection struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Links       []Link   `json:"links"`
	Extent      *Extent  `json:"extent,omitempty"`
	CRS         []string `json:"crs"`
}

// Extent describes the spatial (and optional temporal) extent of a collection.
type Extent struct {
	Spatial SpatialExtent `json:"spatial"`
}

// SpatialExtent holds one or more bounding boxes in CRS84 order [west, south, east, north].
type SpatialExtent struct {
	BBox [][]float64 `json:"bbox"`
	CRS  string      `json:"crs"`
}

// Collections is the /collections response body.
type Collections struct {
	Collections []Collection `json:"collections"`
	Links       []Link       `json:"links"`
}

// Conformance is the /conformance response body.
type Conformance struct {
	ConformsTo []string `json:"conformsTo"`
}

// FeatureCollection is the OGC API Features items response.
type FeatureCollection struct {
	Type            string         `json:"type"`
	Features        []any          `json:"features"`
	NumberMatched   int            `json:"numberMatched"`
	NumberReturned  int            `json:"numberReturned"`
	Links           []Link         `json:"links"`
}
```

### Step 1: Write failing conformance test

```go
// internal/ogcapi/handler_test.go
package ogcapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/meko-tech/lgln-citygml-proxy/internal/ogcapi"
	"github.com/meko-tech/lgln-citygml-proxy/internal/proxy"
)

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	return ogcapi.New(proxy.New(t.TempDir()))
}

func TestConformance(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/conformance", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body ogcapi.Conformance
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.ConformsTo) == 0 {
		t.Error("conformsTo is empty")
	}
}
```

### Step 2: Run — expect FAIL (package doesn't exist)

```bash
go test ./internal/ogcapi/... 2>&1
```

### Step 3: Implement skeleton + conformance handler

```go
// internal/ogcapi/handler.go
package ogcapi

import (
	"encoding/json"
	"net/http"

	"github.com/meko-tech/lgln-citygml-proxy/internal/proxy"
)

// New returns an http.Handler for the OGC API Features sub-router.
func New(fetcher *proxy.Fetcher) http.Handler {
	h := &handler{fetcher: fetcher}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /conformance", h.handleConformance)
	mux.HandleFunc("GET /collections", h.handleCollections)
	mux.HandleFunc("GET /collections/buildings", h.handleCollection)
	mux.HandleFunc("GET /collections/buildings/items", h.handleItems)
	return mux
}

type handler struct {
	fetcher *proxy.Fetcher
}

var conformanceClasses = []string{
	"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/core",
	"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/geojson",
}

func (h *handler) handleConformance(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, Conformance{ConformsTo: conformanceClasses})
}

// ... (other handlers implemented in later tasks)
func (h *handler) handleCollections(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
func (h *handler) handleCollection(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
func (h *handler) handleItems(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "encoding error", http.StatusInternalServerError)
	}
}
```

### Step 4: Run tests

```bash
go test ./internal/ogcapi/... -v -run TestConformance
```
Expected: PASS.

### Step 5: Commit

```bash
git add internal/ogcapi/
git commit -m "feat: add OGC API conformance endpoint skeleton"
```

---

## Task 4: Collections Endpoints

**Files:**
- Modify: `internal/ogcapi/handler.go`
- Modify: `internal/ogcapi/handler_test.go`

### Step 1: Write failing tests

```go
// Add to handler_test.go

func TestCollections(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/collections", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body ogcapi.Collections
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Collections) == 0 {
		t.Fatal("no collections returned")
	}
	found := false
	for _, c := range body.Collections {
		if c.ID == "buildings" {
			found = true
		}
	}
	if !found {
		t.Error("collection 'buildings' not found")
	}
}

func TestCollection_Buildings(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/collections/buildings", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body ogcapi.Collection
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != "buildings" {
		t.Errorf("id = %q, want 'buildings'", body.ID)
	}
}
```

### Step 2: Run — expect FAIL

```bash
go test ./internal/ogcapi/... -v -run "TestCollections|TestCollection_Buildings"
```

### Step 3: Implement collections handlers

```go
// Replace stub handleCollections and handleCollection in handler.go

var buildingsCollection = Collection{
	ID:          "buildings",
	Title:       "LoD2 Buildings (LGLN Niedersachsen)",
	Description: "3D building models for Lower Saxony from LGLN, license CC BY 4.0.",
	Links: []Link{
		{Href: "/collections/buildings/items", Rel: "items", Type: "application/geo+json", Title: "Buildings"},
	},
	Extent: &Extent{
		Spatial: SpatialExtent{
			BBox: [][]float64{{5.9, 51.3, 11.6, 53.9}},
			CRS:  "http://www.opengis.net/def/crs/OGC/1.3/CRS84",
		},
	},
	CRS: []string{"http://www.opengis.net/def/crs/OGC/1.3/CRS84"},
}

func (h *handler) handleCollections(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, Collections{
		Collections: []Collection{buildingsCollection},
		Links:       []Link{{Href: "/collections", Rel: "self", Type: "application/json"}},
	})
}

func (h *handler) handleCollection(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, buildingsCollection)
}
```

### Step 4: Run tests

```bash
go test ./internal/ogcapi/... -v -run "TestCollections|TestCollection_Buildings"
```
Expected: PASS.

### Step 5: Commit

```bash
git add internal/ogcapi/handler.go internal/ogcapi/handler_test.go
git commit -m "feat: add OGC API collections endpoints"
```

---

## Task 5: Items Endpoint (bbox → tiles → GeoJSON)

**Files:**
- Modify: `internal/ogcapi/handler.go`
- Modify: `internal/ogcapi/handler_test.go`

This is the core of Phase 3. The endpoint:
1. Requires `bbox` query parameter (EPSG:25832 meters)
2. Applies tile limit (maxTiles=100)
3. Fetches tiles in parallel via `fetcher.GetMulti`
4. Parses each tile with `citygml.Read`
5. Applies `utm.TransformBuilding` to each building
6. Converts to GeoJSON features via `geojson.BuildingFeature`
7. Paginates with `limit` (default 100, max 1000) and `offset` (default 0)
8. Returns OGC API FeatureCollection

### Step 1: Write failing tests

```go
// Add to handler_test.go — needs a mock upstream that returns minimal CityGML

import (
	"io"
	"net/http/httptest"
)

// minimalCityGML returns a tiny valid CityGML 2.0 document with one building.
// Coordinates are in EPSG:25832 (UTM Zone 32N).
const minimalCityGML = `<?xml version="1.0" encoding="UTF-8"?>
<CityModel xmlns="http://www.opengis.net/citygml/2.0"
           xmlns:bldg="http://www.opengis.net/citygml/building/2.0"
           xmlns:gml="http://www.opengis.net/gml"
           gml:id="test"
           xsi:schemaLocation="http://www.opengis.net/citygml/2.0 CityGML.xsd"
           xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <gml:boundedBy>
    <gml:Envelope srsName="EPSG:25832">
      <gml:lowerCorner>550000 5800000 0</gml:lowerCorner>
      <gml:upperCorner>551000 5801000 50</gml:upperCorner>
    </gml:Envelope>
  </gml:boundedBy>
  <cityObjectMember>
    <bldg:Building gml:id="building-1">
      <bldg:measuredHeight>10.0</bldg:measuredHeight>
      <bldg:lod2MultiSurface>
        <gml:MultiSurface>
          <gml:surfaceMember>
            <gml:Polygon gml:id="poly-1">
              <gml:exterior>
                <gml:LinearRing>
                  <gml:posList>550000 5800000 0 550100 5800000 0 550100 5800100 0 550000 5800000 0</gml:posList>
                </gml:LinearRing>
              </gml:exterior>
            </gml:Polygon>
          </gml:surfaceMember>
        </gml:MultiSurface>
      </bldg:lod2MultiSurface>
    </bldg:Building>
  </cityObjectMember>
</CityModel>`

func upstreamCityGML(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/gml+xml")
		_, _ = io.WriteString(w, body)
	}))
}

func TestItems_RequiresBBox(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/collections/buildings/items", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestItems_TooManyTiles(t *testing.T) {
	h := newHandler(t)
	// 101x101 km → 10201 tiles
	req := httptest.NewRequest(http.MethodGet,
		"/collections/buildings/items?bbox=400000,5700000,500000,5800000", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestItems_OK(t *testing.T) {
	upstream := upstreamCityGML(t, minimalCityGML)
	defer upstream.Close()

	fetcher := proxy.NewWithBaseURL(t.TempDir(), upstream.URL)
	h := ogcapi.New(fetcher)

	// Single tile bbox: 550000–550999 / 5800000–5800999
	req := httptest.NewRequest(http.MethodGet,
		"/collections/buildings/items?bbox=550000,5800000,550999,5800999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/geo+json" {
		t.Errorf("Content-Type = %q, want application/geo+json", ct)
	}
	var body ogcapi.FeatureCollection
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Type != "FeatureCollection" {
		t.Errorf("type = %q, want FeatureCollection", body.Type)
	}
	if body.NumberMatched < 1 {
		t.Errorf("numberMatched = %d, want >= 1", body.NumberMatched)
	}
}

func TestItems_Pagination(t *testing.T) {
	upstream := upstreamCityGML(t, minimalCityGML)
	defer upstream.Close()

	fetcher := proxy.NewWithBaseURL(t.TempDir(), upstream.URL)
	h := ogcapi.New(fetcher)

	req := httptest.NewRequest(http.MethodGet,
		"/collections/buildings/items?bbox=550000,5800000,550999,5800999&limit=0&offset=999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body ogcapi.FeatureCollection
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// limit=0 should default to 100; offset beyond total → 0 features returned
	if body.NumberReturned != 0 {
		t.Errorf("numberReturned = %d, want 0 (offset beyond total)", body.NumberReturned)
	}
}
```

### Step 2: Run — expect FAIL

```bash
go test ./internal/ogcapi/... -v -run "TestItems"
```

### Step 3: Implement the items handler

```go
// Add to handler.go

import (
	"bytes"
	"errors"
	"log"
	"strconv"

	citygml "github.com/cwbudde/go-citygml/citygml"
	cgjson  "github.com/cwbudde/go-citygml/geojson"
	"github.com/meko-tech/lgln-citygml-proxy/internal/bbox"
	"github.com/meko-tech/lgln-citygml-proxy/internal/proxy"
	"github.com/meko-tech/lgln-citygml-proxy/internal/utm"
)

const maxTilesItems = 100

func (h *handler) handleItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	bboxStr := q.Get("bbox")
	if bboxStr == "" {
		http.Error(w, "missing bbox query parameter", http.StatusBadRequest)
		return
	}

	bb, err := bbox.Parse(bboxStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	coords := bb.TileCoords()
	if len(coords) > maxTilesItems {
		http.Error(w, fmt.Sprintf("bbox too large: %d tiles (max %d)", len(coords), maxTilesItems), http.StatusBadRequest)
		return
	}

	// Pagination parameters.
	limit := parseIntParam(q.Get("limit"), 100)
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	offset := parseIntParam(q.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	// Fetch tiles in parallel.
	results := h.fetcher.GetMulti(coords, 4)

	// Parse each tile and collect all buildings.
	var allFeatures []any
	for _, res := range results {
		if res.Err != nil {
			if !errors.Is(res.Err, proxy.ErrNotFound) {
				log.Printf("ogcapi: fetch tile %v: %v", res.Coord, res.Err)
			}
			continue
		}

		doc, err := citygml.Read(bytes.NewReader(res.Data), citygml.Options{})
		if err != nil {
			log.Printf("ogcapi: parse tile %v: %v", res.Coord, err)
			continue
		}

		for i := range doc.Buildings {
			transformed := utm.TransformBuilding(doc.Buildings[i])
			f := cgjson.BuildingFeature(&transformed)
			allFeatures = append(allFeatures, f)
		}
	}

	numberMatched := len(allFeatures)

	// Apply pagination.
	if offset >= len(allFeatures) {
		allFeatures = nil
	} else {
		allFeatures = allFeatures[offset:]
		if len(allFeatures) > limit {
			allFeatures = allFeatures[:limit]
		}
	}

	// Build links.
	selfURL := r.URL.String()
	links := []Link{{Href: selfURL, Rel: "self", Type: "application/geo+json"}}

	w.Header().Set("Content-Type", "application/geo+json")
	writeJSON(w, FeatureCollection{
		Type:           "FeatureCollection",
		Features:       allFeatures,
		NumberMatched:  numberMatched,
		NumberReturned: len(allFeatures),
		Links:          links,
	})
}

func parseIntParam(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
```

**Note:** The `FeatureCollection.Features` field is `[]any` so we can store `cgjson.Feature` values (which are structs, not pointers) without importing the geojson package in the response type.

### Step 4: Run tests

```bash
go test ./internal/ogcapi/... -v -run "TestItems"
```
Expected: all PASS.

Fix any import issues. Common pitfall: the import alias for `citygml` package — use:
```go
import (
	gocitygml "github.com/cwbudde/go-citygml/citygml"
	cgjson    "github.com/cwbudde/go-citygml/geojson"
)
```

### Step 5: Commit

```bash
git add internal/ogcapi/handler.go internal/ogcapi/handler_test.go
git commit -m "feat: add OGC API items endpoint with CityGML→GeoJSON conversion"
```

---

## Task 6: Wire OGC API into Server

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

The existing server uses a single `http.ServeMux`. We add the `/collections/...` routes by mounting the `ogcapi` sub-handler.

### Step 1: Write failing test

```go
// Add to server_test.go

func TestOGCAPI_Conformance_ReachableFromServer(t *testing.T) {
	h := New(proxy.New(t.TempDir()))
	req := httptest.NewRequest(http.MethodGet, "/conformance", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestOGCAPI_Collections_ReachableFromServer(t *testing.T) {
	h := New(proxy.New(t.TempDir()))
	req := httptest.NewRequest(http.MethodGet, "/collections", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
```

### Step 2: Run — expect FAIL (routes not registered)

```bash
go test ./internal/server/... -v -run "TestOGCAPI"
```

### Step 3: Mount ogcapi handler in server

```go
// Modify internal/server/server.go

import "github.com/meko-tech/lgln-citygml-proxy/internal/ogcapi"

func New(fetcher *proxy.Fetcher) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /lod2", handleBBox(fetcher))
	mux.HandleFunc("GET /lod2/{easting}/{northing}", handleTile(fetcher))
	mux.HandleFunc("GET /health", handleHealth)

	// OGC API Features — mount the sub-router at root.
	// The sub-router handles /conformance, /collections, /collections/buildings, /collections/buildings/items.
	ogcHandler := ogcapi.New(fetcher)
	mux.Handle("/conformance", ogcHandler)
	mux.Handle("/collections", ogcHandler)
	mux.Handle("/collections/", ogcHandler)

	return mux
}
```

**Note:** Go 1.22+ `net/http.ServeMux` pattern routing requires careful delegation. Mount the ogcapi handler for the three path prefixes, letting it handle sub-routes internally. Alternatively, extract individual route handlers from the ogcapi package and register them directly.

If the delegation approach causes routing conflicts, replace with direct registration:

```go
// Alternative: export individual handlers from ogcapi and register directly.
// Add to ogcapi package:
// func (h *handler) Conformance() http.HandlerFunc { return h.handleConformance }
// func (h *handler) Collections() http.HandlerFunc { return h.handleCollections }
// func (h *handler) Collection() http.HandlerFunc { return h.handleCollection }
// func (h *handler) Items() http.HandlerFunc { return h.handleItems }

// In server.go:
ogc := ogcapi.NewHandler(fetcher)
mux.HandleFunc("GET /conformance", ogc.Conformance())
mux.HandleFunc("GET /collections", ogc.Collections())
mux.HandleFunc("GET /collections/buildings", ogc.Collection())
mux.HandleFunc("GET /collections/buildings/items", ogc.Items())
```

Use whichever approach compiles cleanly. The exported-handlers approach is simpler and more explicit.

### Step 4: Run all tests

```bash
go test ./... -v 2>&1 | tail -30
```
Expected: all PASS.

### Step 5: Commit

```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "feat: mount OGC API Features routes in server"
```

---

## Task 7: Final Verification

### Step 1: Run full test suite with coverage

```bash
cd /mnt/projekte/Code/lgln-citygml-proxy
go test ./... -count=1 2>&1
```
Expected: all packages PASS.

### Step 2: Vet and lint

```bash
go vet ./...
```

### Step 3: Update PLAN.md

Mark Phase 3 as ✅ complete in `PLAN.md`.

```bash
git add PLAN.md
git commit -m "docs: mark Phase 3 OGC API Features as complete"
```

---

## Appendix: Expected Endpoint Behaviour

| Endpoint | Method | Success | Notes |
|----------|--------|---------|-------|
| `/conformance` | GET | 200 JSON | OGC API Part 1 + GeoJSON |
| `/collections` | GET | 200 JSON | Lists "buildings" collection |
| `/collections/buildings` | GET | 200 JSON | Collection metadata |
| `/collections/buildings/items?bbox=...` | GET | 200 GeoJSON | Features paginated |
| `/collections/buildings/items` | GET | 400 | bbox required |
| `/collections/buildings/items?bbox=<huge>` | GET | 400 | >100 tiles |

## Appendix: CityGML Parse Quirk

`citygml.Read()` is lenient by default (`Options{}` → `Strict: false`). Documents without `srsName` parse fine but `doc.CRS.Code` will be 0. The LGLN tiles always include `EPSG:25832` so this is not a concern, but the handler should tolerate it gracefully (coordinates just won't be meaningful).
