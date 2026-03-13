// Package ogcapi provides an OGC API – Features compliant HTTP handler.
package ogcapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	gocitygml "github.com/cwbudde/go-citygml/citygml"
	cgjson "github.com/cwbudde/go-citygml/geojson"
	"github.com/meko-tech/lgln-citygml-proxy/internal/bbox"
	"github.com/meko-tech/lgln-citygml-proxy/internal/proxy"
	"github.com/meko-tech/lgln-citygml-proxy/internal/utm"
)

// Link represents an OGC API hypermedia link.
type Link struct {
	Href  string `json:"href"`
	Rel   string `json:"rel"`
	Type  string `json:"type,omitempty"`
	Title string `json:"title,omitempty"`
}

// Collection represents a single OGC API feature collection.
type Collection struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Links       []Link   `json:"links"`
	Extent      *Extent  `json:"extent,omitempty"`
	CRS         []string `json:"crs"`
}

// Extent describes the spatial extent of a collection.
type Extent struct {
	Spatial SpatialExtent `json:"spatial"`
}

// SpatialExtent holds bounding box and CRS information for a spatial extent.
type SpatialExtent struct {
	BBox [][]float64 `json:"bbox"`
	CRS  string      `json:"crs"`
}

// Collections is the response type for GET /collections.
type Collections struct {
	Collections []Collection `json:"collections"`
	Links       []Link       `json:"links"`
}

// Conformance is the response type for GET /conformance.
type Conformance struct {
	ConformsTo []string `json:"conformsTo"`
}

// FeatureCollection is the response type for GET /collections/{id}/items.
type FeatureCollection struct {
	Type           string `json:"type"`
	Features       []any  `json:"features"`
	NumberMatched  int    `json:"numberMatched"`
	NumberReturned int    `json:"numberReturned"`
	Links          []Link `json:"links"`
}

// buildingsCollection is the static metadata for the buildings collection.
// Do not mutate this variable; handlers must not modify its slice or pointer fields.
var buildingsCollection = Collection{
	ID:          "buildings",
	Title:       "LoD2 Buildings (LGLN Niedersachsen)",
	Description: "3D building models for Lower Saxony from LGLN, license CC BY 4.0.",
	Links: []Link{
		{Href: "/collections/buildings/items", Rel: "items", Type: "application/geo+json", Title: "Buildings"},
	},
	Extent: &Extent{
		Spatial: SpatialExtent{
			BBox: [][]float64{{5.9, 51.3, 11.6, 53.9}},
			CRS:  "http://www.opengis.net/def/crs/OGC/1.3/CRS84",
		},
	},
	CRS: []string{"http://www.opengis.net/def/crs/OGC/1.3/CRS84"},
}

type handler struct {
	fetcher *proxy.Fetcher
}

// New returns an http.Handler with all OGC API routes registered.
func New(fetcher *proxy.Fetcher) http.Handler {
	h := &handler{fetcher: fetcher}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /conformance", h.handleConformance)
	mux.HandleFunc("GET /collections", h.handleCollections)
	mux.HandleFunc("GET /collections/buildings", h.handleCollection)
	mux.HandleFunc("GET /collections/buildings/items", h.handleItems)
	return mux
}

func (h *handler) handleConformance(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, Conformance{
		ConformsTo: []string{
			"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/core",
			"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/geojson",
		},
	})
}

func (h *handler) handleCollections(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, Collections{
		Collections: []Collection{buildingsCollection},
		Links:       []Link{{Href: "/collections", Rel: "self", Type: "application/json"}},
	})
}

func (h *handler) handleCollection(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, buildingsCollection)
}

func (h *handler) handleItems(w http.ResponseWriter, r *http.Request) {
	bboxStr := r.URL.Query().Get("bbox")
	if bboxStr == "" {
		http.Error(w, "bbox parameter is required", http.StatusBadRequest)
		return
	}

	bb, err := bbox.Parse(bboxStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid bbox: %v", err), http.StatusBadRequest)
		return
	}

	coords := bb.TileCoords()
	if len(coords) > 100 {
		http.Error(w, fmt.Sprintf("bbox too large: %d tiles (max 100)", len(coords)), http.StatusBadRequest)
		return
	}

	results := h.fetcher.GetMulti(coords, 4)

	var features []any
	for _, res := range results {
		if res.Err != nil {
			if !errors.Is(res.Err, proxy.ErrNotFound) {
				log.Printf("tile [%d,%d]: %v", res.Coord[0], res.Coord[1], res.Err)
			}
			continue
		}

		doc, err := gocitygml.Read(bytes.NewReader(res.Data), gocitygml.Options{})
		if err != nil {
			log.Printf("parse tile [%d,%d]: %v", res.Coord[0], res.Coord[1], err)
			continue
		}

		for i := range doc.Buildings {
			transformed := utm.TransformBuilding(doc.Buildings[i])
			feat := cgjson.BuildingFeature(&transformed)
			features = append(features, feat)
		}
	}

	// Pagination
	limit := parseIntParam(r.URL.Query().Get("limit"), 100)
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	offset := parseIntParam(r.URL.Query().Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	total := len(features)
	var page []any
	if offset >= total {
		page = []any{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		page = features[offset:end]
	}

	fc := FeatureCollection{
		Type:           "FeatureCollection",
		Features:       page,
		NumberMatched:  total,
		NumberReturned: len(page),
		Links: []Link{
			{Href: r.URL.RequestURI(), Rel: "self", Type: "application/geo+json", Title: "This document"},
		},
	}

	w.Header().Set("Content-Type", "application/geo+json")
	if err := json.NewEncoder(w).Encode(fc); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// parseIntParam parses an integer query parameter. Returns def if s is empty or invalid.
func parseIntParam(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// writeJSON serialises v as JSON and writes it to w with Content-Type application/json.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
