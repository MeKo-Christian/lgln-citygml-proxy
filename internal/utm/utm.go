// Package utm converts UTM Zone 32N (EPSG:25832) coordinates to WGS84 lon/lat.
package utm

import (
	"math"

	"github.com/cwbudde/go-citygml/types"
)

// WGS84 ellipsoid constants.
const (
	a   = 6378137.0
	f   = 1.0 / 298.257223563
	e2  = 2*f - f*f     // first eccentricity squared
	ep2 = e2 / (1 - e2) // second eccentricity squared
)

// UTM Zone 32N parameters.
const (
	k0      = 0.9996
	falseE  = 500000.0
	falseN  = 0.0
	lambda0 = 9.0 * math.Pi / 180.0 // central meridian in radians
)

// ToWGS84 converts UTM Zone 32N (EPSG:25832) easting/northing in metres to
// WGS84 longitude and latitude in decimal degrees.
// Valid for the northern hemisphere only (falseN = 0).
// Uses Snyder's Transverse Mercator inverse formulas (Snyder p.60-61).
func ToWGS84(easting, northing float64) (lon, lat float64) {
	x := easting - falseE
	y := northing - falseN

	M := y / k0

	// Series coefficients for meridional arc
	aMu := a * (1 - e2/4 - 3*e2*e2/64 - 5*e2*e2*e2/256)
	mu := M / aMu

	e1 := (1 - math.Sqrt(1-e2)) / (1 + math.Sqrt(1-e2))

	// Footprint latitude (phi1) via series
	phi1 := mu +
		(3*e1/2-27*e1*e1*e1/32)*math.Sin(2*mu) +
		(21*e1*e1/16-55*e1*e1*e1*e1/32)*math.Sin(4*mu) +
		(151*e1*e1*e1/96)*math.Sin(6*mu) +
		(1097*e1*e1*e1*e1/512)*math.Sin(8*mu)

	sinPhi1 := math.Sin(phi1)
	cosPhi1 := math.Cos(phi1)
	tanPhi1 := math.Tan(phi1)

	N1 := a / math.Sqrt(1-e2*sinPhi1*sinPhi1)
	T1 := tanPhi1 * tanPhi1
	C1 := ep2 * cosPhi1 * cosPhi1
	R1 := a * (1 - e2) / math.Pow(1-e2*sinPhi1*sinPhi1, 1.5)
	D := x / (N1 * k0)

	D2 := D * D
	D3 := D2 * D
	D4 := D3 * D
	D5 := D4 * D
	D6 := D5 * D

	latRad := phi1 - (N1*tanPhi1/R1)*(D2/2-
		(5+3*T1+10*C1-4*C1*C1-9*ep2)*D4/24+
		(61+90*T1+298*C1+45*T1*T1-252*ep2-3*C1*C1)*D6/720)

	lonRad := lambda0 + (D-
		(1+2*T1+C1)*D3/6+
		(5-2*C1+28*T1-3*C1*C1+8*ep2+24*T1*T1)*D5/120)/cosPhi1

	lon = lonRad * 180.0 / math.Pi
	lat = latRad * 180.0 / math.Pi
	return
}

// transformPoint applies ToWGS84 to p.X (easting) and p.Y (northing).
// p.Z (elevation) is preserved unchanged.
func transformPoint(p types.Point) types.Point {
	lon, lat := ToWGS84(p.X, p.Y)
	return types.Point{X: lon, Y: lat, Z: p.Z}
}

// transformRing returns a new Ring with every point transformed.
func transformRing(r types.Ring) types.Ring {
	out := types.Ring{Points: make([]types.Point, len(r.Points))}
	for i, p := range r.Points {
		out.Points[i] = transformPoint(p)
	}
	return out
}

// transformPolygon returns a new Polygon with exterior and interior rings transformed.
func transformPolygon(poly types.Polygon) types.Polygon {
	out := types.Polygon{
		Exterior: transformRing(poly.Exterior),
	}
	if len(poly.Interior) > 0 {
		out.Interior = make([]types.Ring, len(poly.Interior))
		for i, r := range poly.Interior {
			out.Interior[i] = transformRing(r)
		}
	}
	return out
}

// transformMultiSurface returns a new MultiSurface with every polygon transformed.
func transformMultiSurface(ms types.MultiSurface) types.MultiSurface {
	out := types.MultiSurface{Polygons: make([]types.Polygon, len(ms.Polygons))}
	for i, poly := range ms.Polygons {
		out.Polygons[i] = transformPolygon(poly)
	}
	return out
}

// TransformBuilding returns a copy of b with all geometry fields converted from
// UTM Zone 32N (EPSG:25832) to WGS84 lon/lat. Z (elevation) is preserved.
// Nil geometry fields are left nil in the returned copy.
func TransformBuilding(b types.Building) types.Building {
	out := b // copy all scalar/non-geometry fields

	// Footprint
	if b.Footprint != nil {
		fp := transformPolygon(*b.Footprint)
		out.Footprint = &fp
	} else {
		out.Footprint = nil
	}

	// MultiSurface
	if b.MultiSurface != nil {
		ms := transformMultiSurface(*b.MultiSurface)
		out.MultiSurface = &ms
	} else {
		out.MultiSurface = nil
	}

	// Solid
	if b.Solid != nil {
		s := types.Solid{Exterior: transformMultiSurface(b.Solid.Exterior)}
		out.Solid = &s
	} else {
		out.Solid = nil
	}

	// BoundedBy
	if len(b.BoundedBy) > 0 {
		out.BoundedBy = make([]types.Surface, len(b.BoundedBy))
		for i, surf := range b.BoundedBy {
			out.BoundedBy[i] = types.Surface{
				ID:       surf.ID,
				Type:     surf.Type,
				Geometry: transformMultiSurface(surf.Geometry),
			}
		}
	} else {
		out.BoundedBy = nil
	}

	return out
}
