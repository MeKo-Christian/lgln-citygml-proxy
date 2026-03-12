package utm_test

import (
	"math"
	"testing"

	"github.com/meko-tech/lgln-citygml-proxy/internal/utm"
)

// TestToWGS84_Hannover tests a point near Hannover, Germany.
// easting=550000, northing=5800000 in EPSG:25832 corresponds to
// approximately lon=9.734°, lat=52.348° in WGS84.
// (Note: the task description stated lon≈9.641° which is incorrect;
// forward-projecting 9.641°,52.348° gives easting≈543663, not 550000.)
func TestToWGS84_Hannover(t *testing.T) {
	lon, lat := utm.ToWGS84(550000, 5800000)
	if math.Abs(lon-9.734) > 0.01 {
		t.Errorf("lon = %.6f, want ≈9.734° (tolerance 0.01°)", lon)
	}
	if math.Abs(lat-52.348) > 0.01 {
		t.Errorf("lat = %.6f, want ≈52.348° (tolerance 0.01°)", lat)
	}
}

func TestToWGS84_Origin(t *testing.T) {
	lon, lat := utm.ToWGS84(500000, 0)
	if math.Abs(lon-9.0) > 0.0001 {
		t.Errorf("lon = %.6f, want 9.0° (tolerance 0.0001°)", lon)
	}
	if math.Abs(lat-0.0) > 0.0001 {
		t.Errorf("lat = %.6f, want 0.0° (tolerance 0.0001°)", lat)
	}
}
