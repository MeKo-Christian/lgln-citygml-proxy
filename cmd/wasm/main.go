//go:build js && wasm

package main

import (
	"bytes"
	"encoding/json"
	"syscall/js"

	"github.com/cwbudde/go-citygml/citygml"
	"github.com/cwbudde/go-citygml/geojson"
	"github.com/cwbudde/go-citygml/helpers"
	"github.com/cwbudde/go-citygml/types"
)

func main() {
	js.Global().Set("parseCityGML", js.FuncOf(parseCityGML))
	select {}
}

func parseCityGML(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return errorResult("parseCityGML requires a Uint8Array argument")
	}

	ua := args[0]
	length := ua.Get("length").Int()
	buf := make([]byte, length)
	js.CopyBytesToGo(buf, ua)

	doc, err := citygml.Read(bytes.NewReader(buf), citygml.Options{})
	if err != nil {
		return errorResult(err.Error())
	}

	findings := citygml.Validate(doc)
	fc := geojson.FromDocument(doc)
	bbox := helpers.DocumentBBox(doc)

	cx := (bbox.MinX + bbox.MaxX) / 2
	cy := (bbox.MinY + bbox.MaxY) / 2
	cz := (bbox.MinZ + bbox.MaxZ) / 2

	result := map[string]any{
		"success": true,
		"geojson": marshalAny(fc),
		"scene":   buildScene(doc, cx, cy, cz),
		"meta": map[string]any{
			"version":       doc.Version,
			"srsName":       doc.SRSName,
			"epsgCode":      doc.CRS.Code,
			"buildingCount": len(doc.Buildings),
			"terrainCount":  len(doc.Terrains),
			"genericCount":  len(doc.GenericObjects),
		},
		"objects":  buildObjects(doc),
		"findings": buildFindings(findings),
		"bounds": map[string]any{
			"minX":      bbox.MinX - cx,
			"minY":      bbox.MinY - cy,
			"minZ":      bbox.MinZ - cz,
			"maxX":      bbox.MaxX - cx,
			"maxY":      bbox.MaxY - cy,
			"maxZ":      bbox.MaxZ - cz,
			"centroidX": cx,
			"centroidY": cy,
			"centroidZ": cz,
			"has3D":     bbox.Has3D,
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		return errorResult(err.Error())
	}

	return js.Global().Get("JSON").Call("parse", string(data))
}

func errorResult(msg string) any {
	data, _ := json.Marshal(map[string]any{"success": false, "error": msg})
	return js.Global().Get("JSON").Call("parse", string(data))
}

func marshalAny(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return data
}

func buildScene(doc *types.Document, cx, cy, cz float64) map[string]any {
	var objects []any

	for i := range doc.Buildings {
		b := &doc.Buildings[i]
		obj := map[string]any{
			"id":   b.ID,
			"type": "Building",
		}

		height := b.MeasuredHeight
		if !b.HasMeasuredHeight {
			height = b.DerivedHeight
		}
		obj["height"] = height

		var surfaces []any
		switch {
		case len(b.BoundedBy) > 0:
			for _, surf := range b.BoundedBy {
				surfaces = append(surfaces, map[string]any{
					"type":     surf.Type,
					"polygons": shiftPolygons(surf.Geometry.Polygons, cx, cy, cz),
				})
			}
		case b.Solid != nil:
			surfaces = append(surfaces, map[string]any{
				"type":     "Solid",
				"polygons": shiftPolygons(b.Solid.Exterior.Polygons, cx, cy, cz),
			})
		case b.MultiSurface != nil:
			surfaces = append(surfaces, map[string]any{
				"type":     "MultiSurface",
				"polygons": shiftPolygons(b.MultiSurface.Polygons, cx, cy, cz),
			})
		}

		obj["surfaces"] = surfaces
		objects = append(objects, obj)
	}

	for i := range doc.Terrains {
		t := &doc.Terrains[i]
		objects = append(objects, map[string]any{
			"id":   t.ID,
			"type": "Terrain",
			"surfaces": []any{
				map[string]any{
					"type":     "Terrain",
					"polygons": shiftPolygons(t.Geometry.Polygons, cx, cy, cz),
				},
			},
		})
	}

	return map[string]any{"objects": objects}
}

func shiftPolygons(polys []types.Polygon, cx, cy, cz float64) [][][3]float64 {
	var result [][][3]float64
	for _, poly := range polys {
		var ring [][3]float64
		for _, pt := range poly.Exterior.Points {
			ring = append(ring, [3]float64{pt.X - cx, pt.Y - cy, pt.Z - cz})
		}
		result = append(result, ring)
	}
	return result
}

func buildObjects(doc *types.Document) []map[string]any {
	var objects []map[string]any

	for i := range doc.Buildings {
		b := &doc.Buildings[i]
		obj := map[string]any{
			"id":   b.ID,
			"type": "Building",
		}
		if b.Class != "" {
			obj["class"] = b.Class
		}
		if b.Function != "" {
			obj["function"] = b.Function
		}
		if b.Usage != "" {
			obj["usage"] = b.Usage
		}
		if b.HasMeasuredHeight {
			obj["height"] = b.MeasuredHeight
			obj["heightSource"] = "measured"
		} else if b.DerivedHeight > 0 {
			obj["height"] = b.DerivedHeight
			obj["heightSource"] = "derived"
		}
		if b.LoD != "" {
			obj["lod"] = string(b.LoD)
		}
		objects = append(objects, obj)
	}

	for i := range doc.Terrains {
		objects = append(objects, map[string]any{
			"id":   doc.Terrains[i].ID,
			"type": "Terrain",
		})
	}

	for i := range doc.GenericObjects {
		objects = append(objects, map[string]any{
			"id":   doc.GenericObjects[i].ID,
			"type": doc.GenericObjects[i].Type,
		})
	}

	return objects
}

func buildFindings(findings []citygml.Finding) []map[string]string {
	result := make([]map[string]string, len(findings))
	for i, f := range findings {
		result[i] = map[string]string{
			"severity": f.Severity.String(),
			"path":     f.Path,
			"message":  f.Message,
		}
	}
	return result
}
