// Package proxy fetches and caches LGLN CityGML LoD2 tiles.
package proxy

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const baseURL = "https://lod2.s3.eu-de.cloud-object-storage.appdomain.cloud"

// Fetcher retrieves CityGML tiles, caching them on disk.
type Fetcher struct {
	cacheDir string
	base     string
	client   *http.Client
}

// New creates a Fetcher with the given cache directory.
func New(cacheDir string) *Fetcher {
	return &Fetcher{
		cacheDir: cacheDir,
		base:     baseURL,
		client:   &http.Client{},
	}
}

// NewWithBaseURL creates a Fetcher with an overridden base URL (for testing).
func NewWithBaseURL(cacheDir, base string) *Fetcher {
	return &Fetcher{cacheDir: cacheDir, base: base, client: &http.Client{}}
}

// TileURL returns the upstream URL for the given tile coordinates.
func TileURL(eastingKM, northingKM int) string {
	return fmt.Sprintf("%s/LoD2_32_%d_%d_1_ni.gml", baseURL, eastingKM, northingKM)
}

// tilePath returns the local cache path for a tile.
func (f *Fetcher) tilePath(eastingKM, northingKM int) string {
	name := fmt.Sprintf("LoD2_32_%d_%d_1_ni.gml", eastingKM, northingKM)
	return filepath.Join(f.cacheDir, name)
}

// Get returns the CityGML data for the tile at the given km coordinates.
// It serves from cache if available, otherwise fetches from LGLN.
func (f *Fetcher) Get(eastingKM, northingKM int) ([]byte, error) {
	path := f.tilePath(eastingKM, northingKM)

	// Serve from cache.
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}

	// Fetch from upstream.
	url := fmt.Sprintf("%s/LoD2_32_%d_%d_1_ni.gml", f.base, eastingKM, northingKM)

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}

	// Write to cache (best-effort).
	if err := os.MkdirAll(f.cacheDir, 0o750); err == nil {
		_ = os.WriteFile(path, data, 0o640)
	}

	return data, nil
}

// ErrNotFound indicates the requested tile does not exist upstream.
var ErrNotFound = fmt.Errorf("tile not found")
