// Package stac provides a client for the LGLN STAC API.
package stac

import (
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
	if len(parts) < 4 {
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
