package utm_test

import (
	"math"
	"testing"

	"github.com/cwbudde/go-citygml/types"
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

func TestTransformBuilding_FootprintCoords(t *testing.T) {
	// 4 points in EPSG:25832 near Hannover — same anchor as TestToWGS84_Hannover.
	// After transformation each point must have lon in [0,20] and lat in [40,60].
	pts := []types.Point{
		{X: 550000, Y: 5800000, Z: 50},
		{X: 550100, Y: 5800000, Z: 50},
		{X: 550100, Y: 5800100, Z: 50},
		{X: 550000, Y: 5800100, Z: 50},
	}
	b := types.Building{
		ID: "test-building",
		Footprint: &types.Polygon{
			Exterior: types.Ring{Points: pts},
		},
	}

	out := utm.TransformBuilding(b)

	if out.Footprint == nil {
		t.Fatal("TransformBuilding returned nil Footprint")
	}
	for i, p := range out.Footprint.Exterior.Points {
		if p.X < 0 || p.X > 20 {
			t.Errorf("point[%d] X (lon) = %.6f, want in [0, 20]", i, p.X)
		}
		if p.Y < 40 || p.Y > 60 {
			t.Errorf("point[%d] Y (lat) = %.6f, want in [40, 60]", i, p.Y)
		}
		if p.Z != 50 {
			t.Errorf("point[%d] Z = %.6f, want 50 (preserved)", i, p.Z)
		}
	}
}

func TestTransformBuilding_NilFields(t *testing.T) {
	// A building with no geometry fields set must not panic.
	b := types.Building{ID: "empty"}
	out := utm.TransformBuilding(b)
	if out.Footprint != nil {
		t.Error("expected nil Footprint")
	}
	if out.MultiSurface != nil {
		t.Error("expected nil MultiSurface")
	}
	if out.Solid != nil {
		t.Error("expected nil Solid")
	}
	if len(out.BoundedBy) != 0 {
		t.Error("expected empty BoundedBy")
	}
}
