package ogcapi_test

import (
	"encoding/json"
	"io"
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

func newHandlerWithUpstream(t *testing.T, upstream string) http.Handler {
	t.Helper()
	return ogcapi.New(proxy.NewWithBaseURL(t.TempDir(), upstream))
}

const minimalCityGML = `<?xml version="1.0" encoding="UTF-8"?>
<CityModel xmlns="http://www.opengis.net/citygml/2.0"
           xmlns:bldg="http://www.opengis.net/citygml/building/2.0"
           xmlns:gml="http://www.opengis.net/gml"
           gml:id="test">
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

func TestConformance(t *testing.T) {
	h := newHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/conformance", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var conf ogcapi.Conformance
	if err := json.NewDecoder(w.Body).Decode(&conf); err != nil {
		t.Fatalf("failed to decode conformance response: %v", err)
	}

	if len(conf.ConformsTo) == 0 {
		t.Fatal("expected at least one conformance class URI, got none")
	}
}

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

func TestItems_RequiresBBox(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/collections/buildings/items", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestItems_TooManyTiles(t *testing.T) {
	h := newHandler(t)
	// 400000,5700000 to 500000,5800000 → 101×101 = 10201 tiles
	req := httptest.NewRequest(http.MethodGet, "/collections/buildings/items?bbox=400000,5700000,500000,5800000", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestItems_OK(t *testing.T) {
	upstream := upstreamCityGML(t, minimalCityGML)
	defer upstream.Close()

	h := newHandlerWithUpstream(t, upstream.URL)
	// Single tile: 550000,5800000 to 550999,5800999 → tile [550,5800]
	req := httptest.NewRequest(http.MethodGet, "/collections/buildings/items?bbox=550000,5800000,550999,5800999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/geo+json" {
		t.Errorf("Content-Type = %q, want application/geo+json", ct)
	}

	var fc ogcapi.FeatureCollection
	if err := json.NewDecoder(rec.Body).Decode(&fc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if fc.Type != "FeatureCollection" {
		t.Errorf("type = %q, want FeatureCollection", fc.Type)
	}
	if fc.NumberMatched < 1 {
		t.Errorf("numberMatched = %d, want >= 1", fc.NumberMatched)
	}
}

func TestItems_OffsetBeyondTotal(t *testing.T) {
	upstream := upstreamCityGML(t, minimalCityGML)
	defer upstream.Close()

	h := newHandlerWithUpstream(t, upstream.URL)
	req := httptest.NewRequest(http.MethodGet, "/collections/buildings/items?bbox=550000,5800000,550999,5800999&limit=100&offset=999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var fc ogcapi.FeatureCollection
	if err := json.NewDecoder(rec.Body).Decode(&fc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if fc.NumberReturned != 0 {
		t.Errorf("numberReturned = %d, want 0", fc.NumberReturned)
	}
}

func TestItems_LimitZeroDefaultsTo100(t *testing.T) {
	upstream := upstreamCityGML(t, minimalCityGML)
	defer upstream.Close()

	h := newHandlerWithUpstream(t, upstream.URL)
	// limit=0 should default to 100, returning the 1 available feature
	req := httptest.NewRequest(http.MethodGet, "/collections/buildings/items?bbox=550000,5800000,550999,5800999&limit=0", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var fc ogcapi.FeatureCollection
	if err := json.NewDecoder(rec.Body).Decode(&fc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if fc.NumberReturned != fc.NumberMatched {
		t.Errorf("limit=0 should default to 100: numberReturned=%d, numberMatched=%d", fc.NumberReturned, fc.NumberMatched)
	}
}
