package bbox

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// BBox represents a bounding box in EPSG:25832 meter coordinates.
type BBox struct {
	West, South, East, North float64
}

// Parse parses a comma-separated bbox string "west,south,east,north".
func Parse(s string) (BBox, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return BBox{}, fmt.Errorf("bbox must have 4 values, got %d", len(parts))
	}

	vals := make([]float64, 4)
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return BBox{}, fmt.Errorf("invalid bbox value %q: %w", p, err)
		}
		vals[i] = v
	}

	bb := BBox{vals[0], vals[1], vals[2], vals[3]}
	if bb.West > bb.East {
		return BBox{}, fmt.Errorf("bbox west (%v) > east (%v)", bb.West, bb.East)
	}
	if bb.South > bb.North {
		return BBox{}, fmt.Errorf("bbox south (%v) > north (%v)", bb.South, bb.North)
	}
	return bb, nil
}

// TileCoords returns all 1km tile coordinates [easting_km, northing_km]
// that intersect the bounding box.
func (b BBox) TileCoords() [][2]int {
	westKM := int(math.Floor(b.West / 1000))
	southKM := int(math.Floor(b.South / 1000))
	eastKM := int(math.Floor(b.East / 1000))
	northKM := int(math.Floor(b.North / 1000))

	var coords [][2]int
	for e := westKM; e <= eastKM; e++ {
		for n := southKM; n <= northKM; n++ {
			coords = append(coords, [2]int{e, n})
		}
	}
	return coords
}
