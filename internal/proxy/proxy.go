// Package proxy fetches and caches LGLN CityGML LoD2 tiles.
package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/meko-tech/lgln-citygml-proxy/internal/bbox"
	"github.com/meko-tech/lgln-citygml-proxy/internal/stac"
	"github.com/meko-tech/lgln-citygml-proxy/internal/utm"
)

const baseURL = "https://lod2.s3.eu-de.cloud-object-storage.appdomain.cloud"
const lod1BaseURL = "https://lod1.s3.eu-de.cloud-object-storage.appdomain.cloud"

// Fetcher retrieves CityGML tiles, caching them on disk.
type Fetcher struct {
	cacheDir     string
	base         string
	tileTemplate string // printf pattern: "LoD2_32_%d_%d_1_ni.gml" or "LoD1_32_%d_%d_1_ni.gml"
	label        string // "lod2" or "lod1"
	client       *http.Client
	stac         *stac.Client // nil = no STAC integration
}

// New creates a Fetcher with the given cache directory.
func New(cacheDir string) *Fetcher {
	return &Fetcher{
		cacheDir:     cacheDir,
		base:         baseURL,
		tileTemplate: "LoD2_32_%d_%d_1_ni.gml",
		label:        "lod2",
		client:       &http.Client{},
	}
}

// NewWithBaseURL creates a Fetcher with an overridden base URL (for testing).
func NewWithBaseURL(cacheDir, base string) *Fetcher {
	return &Fetcher{
		cacheDir:     cacheDir,
		base:         base,
		tileTemplate: "LoD2_32_%d_%d_1_ni.gml",
		label:        "lod2",
		client:       &http.Client{},
	}
}

// NewWithSTAC creates a Fetcher with the default upstream URL and a STAC client
// pointing at stacBaseURL for cache freshness checks.
func NewWithSTAC(cacheDir, stacBaseURL string) *Fetcher {
	return &Fetcher{
		cacheDir:     cacheDir,
		base:         baseURL,
		tileTemplate: "LoD2_32_%d_%d_1_ni.gml",
		label:        "lod2",
		client:       &http.Client{},
		stac:         stac.New(stacBaseURL),
	}
}

// NewLoD1 creates a Fetcher for LoD1 tiles from the LGLN S3 bucket.
func NewLoD1(cacheDir string) *Fetcher {
	return &Fetcher{
		cacheDir:     cacheDir,
		base:         lod1BaseURL,
		tileTemplate: "LoD1_32_%d_%d_1_ni.gml",
		label:        "lod1",
		client:       &http.Client{},
	}
}

// NewLoD1WithBaseURL creates a LoD1 Fetcher with an overridden base URL (for testing).
func NewLoD1WithBaseURL(cacheDir, base string) *Fetcher {
	return &Fetcher{
		cacheDir:     cacheDir,
		base:         base,
		tileTemplate: "LoD1_32_%d_%d_1_ni.gml",
		label:        "lod1",
		client:       &http.Client{},
	}
}

// TileURL returns the upstream URL for the given tile coordinates.
func TileURL(eastingKM, northingKM int) string {
	return fmt.Sprintf("%s/LoD2_32_%d_%d_1_ni.gml", baseURL, eastingKM, northingKM)
}

// TileName returns the filename for the tile at the given km coordinates.
func (f *Fetcher) TileName(eastingKM, northingKM int) string {
	return fmt.Sprintf(f.tileTemplate, eastingKM, northingKM)
}

// Label returns "lod2" or "lod1", identifying which LoD level this Fetcher serves.
func (f *Fetcher) Label() string {
	return f.label
}

// tilePath returns the local cache path for a tile.
func (f *Fetcher) tilePath(eastingKM, northingKM int) string {
	return filepath.Join(f.cacheDir, f.TileName(eastingKM, northingKM))
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
	url := fmt.Sprintf("%s/%s", f.base, f.TileName(eastingKM, northingKM))

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

// Invalidate removes the cached file for the given tile, if it exists.
// Returns nil if the file does not exist.
func (f *Fetcher) Invalidate(eastingKM, northingKM int) error {
	err := os.Remove(f.tilePath(eastingKM, northingKM))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// invalidateIfStale removes the cached file for the given tile if it is older
// than updatedAt. Errors are silently ignored (best-effort).
func (f *Fetcher) invalidateIfStale(eastingKM, northingKM int, updatedAt time.Time) {
	path := f.tilePath(eastingKM, northingKM)
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.ModTime().Before(updatedAt) {
		_ = os.Remove(path)
	}
}

// BBoxTileCoords returns all 1 km tile coordinates that intersect the given
// EPSG:25832 bounding box. When a STAC client is configured, it also
// invalidates cached tiles that are older than the STAC-reported update time.
func (f *Fetcher) BBoxTileCoords(ctx context.Context, bb bbox.BBox) ([][2]int, error) {
	if f.stac == nil {
		return bb.TileCoords(), nil
	}

	// Convert EPSG:25832 corners to WGS84 for the STAC query.
	westLon, southLat := utm.ToWGS84(bb.West, bb.South)
	eastLon, northLat := utm.ToWGS84(bb.East, bb.North)

	items, err := f.stac.ItemsByBBox(ctx, westLon, southLat, eastLon, northLat)
	if err != nil {
		return nil, fmt.Errorf("stac items: %w", err)
	}

	coords := make([][2]int, 0, len(items))
	for _, item := range items {
		if !item.UpdatedAt.IsZero() {
			f.invalidateIfStale(item.EastingKM, item.NorthingKM, item.UpdatedAt)
		}
		coords = append(coords, [2]int{item.EastingKM, item.NorthingKM})
	}
	return coords, nil
}

// TileResult holds the result of fetching a single tile.
type TileResult struct {
	Coord [2]int
	Data  []byte
	Err   error
}

// GetMulti fetches multiple tiles concurrently with the given concurrency limit.
func (f *Fetcher) GetMulti(coords [][2]int, concurrency int) []TileResult {
	results := make([]TileResult, len(coords))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, c := range coords {
		wg.Add(1)
		go func(i int, c [2]int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			data, err := f.Get(c[0], c[1])
			results[i] = TileResult{Coord: c, Data: data, Err: err}
		}(i, c)
	}

	wg.Wait()
	return results
}
