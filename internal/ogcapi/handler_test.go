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
