package server

import (
	"archive/zip"
	"bytes"
	"fmt"
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
	h := New(proxy.New(t.TempDir()), nil)

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
	h := New(fetcher, nil)

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
	h := New(fetcher, nil)

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
	h := New(fetcher, nil)

	req := httptest.NewRequest(http.MethodGet, "/lod2/999/999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandleTile_BadEasting(t *testing.T) {
	h := New(proxy.New(t.TempDir()), nil)

	req := httptest.NewRequest(http.MethodGet, "/lod2/abc/5800", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleTile_BadNorthing(t *testing.T) {
	h := New(proxy.New(t.TempDir()), nil)

	req := httptest.NewRequest(http.MethodGet, "/lod2/550/xyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleBBox_OK(t *testing.T) {
	upstream := upstreamServer(t, http.StatusOK, "<CityModel/>")
	defer upstream.Close()

	fetcher := proxy.NewWithBaseURL(t.TempDir(), upstream.URL)
	h := New(fetcher, nil)

	req := httptest.NewRequest(http.MethodGet, "/lod2?bbox=550000,5800000,550999,5800999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type = %q, want application/zip", ct)
	}

	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}
	if len(zr.File) != 1 {
		t.Fatalf("zip has %d files, want 1", len(zr.File))
	}
	if zr.File[0].Name != "LoD2_32_550_5800_1_ni.gml" {
		t.Errorf("zip file name = %q, want LoD2_32_550_5800_1_ni.gml", zr.File[0].Name)
	}
}

func TestHandleBBox_MissingParam(t *testing.T) {
	h := New(proxy.New(t.TempDir()), nil)

	req := httptest.NewRequest(http.MethodGet, "/lod2", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleBBox_InvalidParam(t *testing.T) {
	h := New(proxy.New(t.TempDir()), nil)

	req := httptest.NewRequest(http.MethodGet, "/lod2?bbox=invalid", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleBBox_MultipleTiles(t *testing.T) {
	upstream := upstreamServer(t, http.StatusOK, "<CityModel/>")
	defer upstream.Close()

	fetcher := proxy.NewWithBaseURL(t.TempDir(), upstream.URL)
	h := New(fetcher, nil)

	req := httptest.NewRequest(http.MethodGet, "/lod2?bbox=550000,5800000,551999,5801999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}
	if len(zr.File) != 4 {
		t.Errorf("zip has %d files, want 4", len(zr.File))
	}
}

func TestHandleBBox_TooManyTiles(t *testing.T) {
	h := New(proxy.New(t.TempDir()), nil)

	// 101x101 km = 10201 tiles — exceeds maxTiles=100
	req := httptest.NewRequest(http.MethodGet, "/lod2?bbox=400000,5700000,500000,5800000", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleBBox_AllNotFound(t *testing.T) {
	upstream := upstreamServer(t, http.StatusNotFound, "")
	defer upstream.Close()

	fetcher := proxy.NewWithBaseURL(t.TempDir(), upstream.URL)
	h := New(fetcher, nil)

	req := httptest.NewRequest(http.MethodGet, "/lod2?bbox=550000,5800000,550999,5800999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestOGCAPI_Conformance_ReachableFromServer(t *testing.T) {
	h := New(proxy.New(t.TempDir()), nil)
	req := httptest.NewRequest(http.MethodGet, "/conformance", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestOGCAPI_Collections_ReachableFromServer(t *testing.T) {
	h := New(proxy.New(t.TempDir()), nil)
	req := httptest.NewRequest(http.MethodGet, "/collections", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestLoD1_SingleTile(t *testing.T) {
	upstream := upstreamServer(t, http.StatusOK, "<CityModel/>")
	defer upstream.Close()

	lod1 := proxy.NewLoD1WithBaseURL(t.TempDir(), upstream.URL)
	h := New(proxy.New(t.TempDir()), lod1)

	req := httptest.NewRequest(http.MethodGet, "/lod1/550/5800", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/gml+xml" {
		t.Errorf("Content-Type = %q, want application/gml+xml", ct)
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "LoD1_32_550_5800_1_ni.gml") {
		t.Errorf("Content-Disposition = %q, want LoD1 filename", cd)
	}
}

func TestLoD1_BBox(t *testing.T) {
	upstream := upstreamServer(t, http.StatusOK, "<CityModel/>")
	defer upstream.Close()

	lod1 := proxy.NewLoD1WithBaseURL(t.TempDir(), upstream.URL)
	h := New(proxy.New(t.TempDir()), lod1)

	req := httptest.NewRequest(http.MethodGet, "/lod1?bbox=550000,5800000,550999,5800999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type = %q, want application/zip", ct)
	}

	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}
	if len(zr.File) != 1 {
		t.Fatalf("zip has %d files, want 1", len(zr.File))
	}
	if zr.File[0].Name != "LoD1_32_550_5800_1_ni.gml" {
		t.Errorf("zip entry = %q, want LoD1_32_550_5800_1_ni.gml", zr.File[0].Name)
	}
}

func TestLoD1_NotRegisteredWhenNil(t *testing.T) {
	h := New(proxy.New(t.TempDir()), nil)
	req := httptest.NewRequest(http.MethodGet, "/lod1/550/5800", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 or 405 (route not registered)", rec.Code)
	}
}

var _ = fmt.Sprintf // ensure fmt is used
