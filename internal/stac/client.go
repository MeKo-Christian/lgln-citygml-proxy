// Package stac provides a client for the LGLN STAC API.
package stac

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Item represents a single STAC tile item.
type Item struct {
	EastingKM  int
	NorthingKM int
	UpdatedAt  time.Time // zero if timestamp not available in the STAC response
}

// ParseItemID parses a STAC item ID of the form "LoD2_32_{easting}_{northing}_1_ni"
// (or with a .gml suffix) into km tile coordinates.
// Returns ok=false for unrecognised patterns.
func ParseItemID(id string) (eastingKM, northingKM int, ok bool) {
	id = strings.TrimSuffix(id, ".gml")
	parts := strings.Split(id, "_")
	if len(parts) != 6 {
		return 0, 0, false
	}
	if parts[0] != "LoD2" || parts[1] != "32" {
		return 0, 0, false
	}
	e, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, false
	}
	n, err := strconv.Atoi(parts[3])
	if err != nil {
		return 0, 0, false
	}
	if e < 0 || n < 0 {
		return 0, 0, false
	}
	return e, n, true
}

// parseTime parses a STAC timestamp string.
// Supports RFC3339 full timestamps, date-only strings, and 4-digit year strings.
func parseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	if len(s) == 4 {
		if y, err := strconv.Atoi(s); err == nil {
			return time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}

// DefaultBaseURL is the LGLN STAC API base URL.
const DefaultBaseURL = "https://lod.stac.lgln.niedersachsen.de"

// Client queries the LGLN STAC API.
type Client struct {
	base   string
	client *http.Client
}

// New creates a Client with the given STAC base URL (useful for tests).
func New(baseURL string) *Client {
	return &Client{base: baseURL, client: &http.Client{}}
}

// NewDefault creates a Client pointing at the LGLN STAC API.
func NewDefault() *Client {
	return New(DefaultBaseURL)
}

// stacResponse is the minimal JSON structure of a STAC FeatureCollection.
type stacResponse struct {
	Features []stacFeature `json:"features"`
}

type stacFeature struct {
	ID         string         `json:"id"`
	Properties stacProperties `json:"properties"`
}

type stacProperties struct {
	DateTime    string `json:"datetime"`
	Aktualitaet string `json:"Aktualitaet"`
}

// ItemsByBBox queries the STAC API for tiles intersecting the given WGS84 bounding box.
// Unrecognised item IDs are silently skipped.
func (c *Client) ItemsByBBox(ctx context.Context, westLon, southLat, eastLon, northLat float64) ([]Item, error) {
	q := url.Values{
		"bbox":  {fmt.Sprintf("%f,%f,%f,%f", westLon, southLat, eastLon, northLat)},
		"limit": {"500"},
	}
	endpoint := fmt.Sprintf("%s/collections/lod2/items?%s", c.base, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("stac request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stac get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stac: status %d", resp.StatusCode)
	}

	var fc stacResponse
	if err := json.NewDecoder(resp.Body).Decode(&fc); err != nil {
		return nil, fmt.Errorf("stac decode: %w", err)
	}

	items := make([]Item, 0, len(fc.Features))
	for _, f := range fc.Features {
		e, n, ok := ParseItemID(f.ID)
		if !ok {
			continue
		}
		item := Item{EastingKM: e, NorthingKM: n}
		// Aktualitaet takes precedence; fall back to datetime.
		ts := f.Properties.Aktualitaet
		if ts == "" {
			ts = f.Properties.DateTime
		}
		item.UpdatedAt, _ = parseTime(ts)
		items = append(items, item)
	}
	return items, nil
}
