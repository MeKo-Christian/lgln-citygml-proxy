package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
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
		if r.Err == nil {
			found++
		} else if r.Err == ErrNotFound {
			notFound++
		}
	}
	if found != 1 || notFound != 1 {
		t.Errorf("got found=%d notFound=%d, want found=1 notFound=1", found, notFound)
	}
}
