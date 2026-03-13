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
