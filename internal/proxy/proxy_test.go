package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/meko-tech/lgln-citygml-proxy/internal/bbox"
)

func TestTileURL(t *testing.T) {
	tests := []struct {
		easting  int
		northing int
		want     string
	}{
		{550, 5800, "https://lod2.s3.eu-de.cloud-object-storage.appdomain.cloud/LoD2_32_550_5800_1_ni.gml"},
		{551, 5801, "https://lod2.s3.eu-de.cloud-object-storage.appdomain.cloud/LoD2_32_551_5801_1_ni.gml"},
	}
	for _, tt := range tests {
		got := TileURL(tt.easting, tt.northing)
		if got != tt.want {
			t.Errorf("TileURL(%d, %d) = %q, want %q", tt.easting, tt.northing, got, tt.want)
		}
	}
}

func TestFetcher_GetFromCache(t *testing.T) {
	dir := t.TempDir()
	data := []byte("<CityModel/>")
	name := "LoD2_32_550_5800_1_ni.gml"
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o640); err != nil {
		t.Fatal(err)
	}

	f := New(dir)
	got, err := f.Get(550, 5800)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("got %q, want %q", got, data)
	}
}

func TestFetcher_GetFromUpstream(t *testing.T) {
	body := []byte("<CityModel/>")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := NewWithBaseURL(dir, srv.URL)

	got, err := f.Get(550, 5800)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("got %q, want %q", got, body)
	}

	// Verify tile was cached on disk.
	cached := filepath.Join(dir, "LoD2_32_550_5800_1_ni.gml")
	if _, err := os.Stat(cached); os.IsNotExist(err) {
		t.Error("tile not written to cache")
	}
}

func TestFetcher_GetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := NewWithBaseURL(t.TempDir(), srv.URL)
	_, err := f.Get(999, 999)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestFetcher_GetUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := NewWithBaseURL(t.TempDir(), srv.URL)
	_, err := f.Get(550, 5800)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestFetcher_GetMulti(t *testing.T) {
	var mu sync.Mutex
	requested := make(map[string]bool)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requested[r.URL.Path] = true
		mu.Unlock()
		fmt.Fprintf(w, "<CityModel>%s</CityModel>", r.URL.Path)
	}))
	defer srv.Close()

	f := NewWithBaseURL(t.TempDir(), srv.URL)
	coords := [][2]int{{550, 5800}, {551, 5801}}

	results := f.GetMulti(coords, 2)

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("tile %v: unexpected error: %v", r.Coord, r.Err)
		}
		if len(r.Data) == 0 {
			t.Errorf("tile %v: empty data", r.Coord)
		}
	}
}

func TestFetcher_GetMulti_SkipsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "999_999") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprintf(w, "<CityModel/>")
	}))
	defer srv.Close()

	f := NewWithBaseURL(t.TempDir(), srv.URL)
	coords := [][2]int{{550, 5800}, {999, 999}}

	results := f.GetMulti(coords, 2)

	found := 0
	notFound := 0
	for _, r := range results {
		switch r.Err {
		case nil:
			found++
		case ErrNotFound:
			notFound++
		}
	}
	if found != 1 || notFound != 1 {
		t.Errorf("got found=%d notFound=%d, want found=1 notFound=1", found, notFound)
	}
}

func TestFetcher_Invalidate(t *testing.T) {
	dir := t.TempDir()
	name := "LoD2_32_550_5800_1_ni.gml"
	cachePath := filepath.Join(dir, name)
	if err := os.WriteFile(cachePath, []byte("<CityModel/>"), 0o640); err != nil {
		t.Fatal(err)
	}

	f := New(dir)
	if err := f.Invalidate(550, 5800); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("expected cache file to be removed after Invalidate")
	}
}

func TestFetcher_Invalidate_NoFile(t *testing.T) {
	f := New(t.TempDir())
	if err := f.Invalidate(999, 999); err != nil {
		t.Errorf("Invalidate on non-existent tile should not error, got: %v", err)
	}
}

func TestFetcher_BBoxTileCoords_NoSTAC(t *testing.T) {
	f := New(t.TempDir())
	bb := bbox.BBox{West: 550000, South: 5800000, East: 550999, North: 5800999}
	coords, err := f.BBoxTileCoords(context.Background(), bb)
	if err != nil {
		t.Fatalf("BBoxTileCoords: %v", err)
	}
	if len(coords) != 1 {
		t.Fatalf("got %d coords, want 1", len(coords))
	}
	if coords[0] != [2]int{550, 5800} {
		t.Errorf("got %v, want [550 5800]", coords[0])
	}
}

func TestFetcher_BBoxTileCoords_WithSTAC_InvalidatesStale(t *testing.T) {
	dir := t.TempDir()

	// Plant a cache file with a very old mtime (2020).
	name := "LoD2_32_550_5800_1_ni.gml"
	cachePath := filepath.Join(dir, name)
	if err := os.WriteFile(cachePath, []byte("<CityModel/>"), 0o640); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = os.Chtimes(cachePath, oldTime, oldTime)

	// Mock STAC server returning tile 550/5800 with a 2023 update timestamp.
	stacSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body := `{"type":"FeatureCollection","features":[` +
			`{"id":"LoD2_32_550_5800_1_ni","type":"Feature","properties":{"datetime":"2023-01-01T00:00:00Z"}}` +
			`]}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	defer stacSrv.Close()

	f := NewWithSTAC(dir, stacSrv.URL)
	bb := bbox.BBox{West: 550000, South: 5800000, East: 550999, North: 5800999}
	coords, err := f.BBoxTileCoords(context.Background(), bb)
	if err != nil {
		t.Fatalf("BBoxTileCoords: %v", err)
	}

	// Stale cache should have been deleted.
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("expected stale cache file to be removed")
	}
	if len(coords) != 1 || coords[0] != [2]int{550, 5800} {
		t.Errorf("coords = %v, want [[550 5800]]", coords)
	}
}

func TestTileName_LoD2(t *testing.T) {
	f := New(t.TempDir())
	if got := f.TileName(550, 5800); got != "LoD2_32_550_5800_1_ni.gml" {
		t.Errorf("TileName = %q, want LoD2_32_550_5800_1_ni.gml", got)
	}
}

func TestTileName_LoD1(t *testing.T) {
	f := NewLoD1(t.TempDir())
	if got := f.TileName(550, 5800); got != "LoD1_32_550_5800_1_ni.gml" {
		t.Errorf("TileName = %q, want LoD1_32_550_5800_1_ni.gml", got)
	}
}

func TestLabel_LoD2(t *testing.T) {
	f := New(t.TempDir())
	if got := f.Label(); got != "lod2" {
		t.Errorf("Label = %q, want lod2", got)
	}
}

func TestLabel_LoD1(t *testing.T) {
	f := NewLoD1(t.TempDir())
	if got := f.Label(); got != "lod1" {
		t.Errorf("Label = %q, want lod1", got)
	}
}

func TestFetcher_GetLoD1_UsesCorrectURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprint(w, "<CityModel/>")
	}))
	defer srv.Close()

	f := NewLoD1WithBaseURL(t.TempDir(), srv.URL)
	if _, err := f.Get(550, 5800); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if want := "/LoD1_32_550_5800_1_ni.gml"; gotPath != want {
		t.Errorf("fetched path = %q, want %q", gotPath, want)
	}
}

func TestFetcher_BBoxTileCoords_WithSTAC_KeepsFreshCache(t *testing.T) {
	dir := t.TempDir()

	// Plant a cache file with a future mtime (2030 — definitively fresh).
	name := "LoD2_32_550_5800_1_ni.gml"
	cachePath := filepath.Join(dir, name)
	if err := os.WriteFile(cachePath, []byte("<CityModel/>"), 0o640); err != nil {
		t.Fatal(err)
	}
	futureTime := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = os.Chtimes(cachePath, futureTime, futureTime)

	// STAC returns a 2023 update — older than the cache.
	stacSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body := `{"type":"FeatureCollection","features":[` +
			`{"id":"LoD2_32_550_5800_1_ni","type":"Feature","properties":{"datetime":"2023-01-01T00:00:00Z"}}` +
			`]}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	defer stacSrv.Close()

	f := NewWithSTAC(dir, stacSrv.URL)
	bb := bbox.BBox{West: 550000, South: 5800000, East: 550999, North: 5800999}
	if _, err := f.BBoxTileCoords(context.Background(), bb); err != nil {
		t.Fatalf("BBoxTileCoords: %v", err)
	}

	// Cache should still exist.
	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("expected fresh cache file to remain, got: %v", err)
	}
}
