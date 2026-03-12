package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/meko-tech/lgln-citygml-proxy/internal/proxy"
)

// upstreamServer returns a test HTTP server that responds with status/body for any request.
func upstreamServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

func TestHandleHealth(t *testing.T) {
	h := New(proxy.New(t.TempDir()))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ok") {
		t.Errorf("body = %q, want \"ok\"", rec.Body.String())
	}
}

func TestHandleTile_OK(t *testing.T) {
	const tileData = "<CityModel/>"
	upstream := upstreamServer(t, http.StatusOK, tileData)
	defer upstream.Close()

	fetcher := proxy.NewWithBaseURL(t.TempDir(), upstream.URL)
	h := New(fetcher)

	req := httptest.NewRequest(http.MethodGet, "/lod2/550/5800", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != tileData {
		t.Errorf("body = %q, want %q", rec.Body.String(), tileData)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/gml+xml" {
		t.Errorf("Content-Type = %q, want application/gml+xml", ct)
	}
}

func TestHandleTile_GmlSuffix(t *testing.T) {
	upstream := upstreamServer(t, http.StatusOK, "<CityModel/>")
	defer upstream.Close()

	fetcher := proxy.NewWithBaseURL(t.TempDir(), upstream.URL)
	h := New(fetcher)

	req := httptest.NewRequest(http.MethodGet, "/lod2/550/5800.gml", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestHandleTile_NotFound(t *testing.T) {
	upstream := upstreamServer(t, http.StatusNotFound, "")
	defer upstream.Close()

	fetcher := proxy.NewWithBaseURL(t.TempDir(), upstream.URL)
	h := New(fetcher)

	req := httptest.NewRequest(http.MethodGet, "/lod2/999/999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandleTile_BadEasting(t *testing.T) {
	h := New(proxy.New(t.TempDir()))

	req := httptest.NewRequest(http.MethodGet, "/lod2/abc/5800", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleTile_BadNorthing(t *testing.T) {
	h := New(proxy.New(t.TempDir()))

	req := httptest.NewRequest(http.MethodGet, "/lod2/550/xyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
