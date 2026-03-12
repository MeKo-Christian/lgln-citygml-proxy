// Package server provides the HTTP handlers for the CityGML tile proxy.
package server

import (
	"archive/zip"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/meko-tech/lgln-citygml-proxy/internal/bbox"
	"github.com/meko-tech/lgln-citygml-proxy/internal/proxy"
)

// New returns an http.Handler with all routes registered.
func New(fetcher *proxy.Fetcher) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /lod2", handleBBox(fetcher))
	mux.HandleFunc("GET /lod2/{easting}/{northing}", handleTile(fetcher))
	mux.HandleFunc("GET /health", handleHealth)
	return mux
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "ok")
}

func handleTile(fetcher *proxy.Fetcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		eastingStr := r.PathValue("easting")
		northingStr := r.PathValue("northing")

		// Strip .gml suffix if present.
		northingStr = strings.TrimSuffix(northingStr, ".gml")

		easting, err := strconv.Atoi(eastingStr)
		if err != nil {
			http.Error(w, "invalid easting: must be integer (km)", http.StatusBadRequest)
			return
		}

		northing, err := strconv.Atoi(northingStr)
		if err != nil {
			http.Error(w, "invalid northing: must be integer (km)", http.StatusBadRequest)
			return
		}

		data, err := fetcher.Get(easting, northing)
		if err != nil {
			if errors.Is(err, proxy.ErrNotFound) {
				http.Error(w, "tile not found", http.StatusNotFound)
				return
			}

			log.Printf("error fetching tile %d/%d: %v", easting, northing, err)
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/gml+xml")
		w.Header().Set("Content-Disposition",
			fmt.Sprintf("inline; filename=\"LoD2_32_%d_%d_1_ni.gml\"", easting, northing))
		w.Write(data)
	}
}

func handleBBox(fetcher *proxy.Fetcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bboxStr := r.URL.Query().Get("bbox")
		if bboxStr == "" {
			http.Error(w, "missing bbox query parameter", http.StatusBadRequest)
			return
		}

		bb, err := bbox.Parse(bboxStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		coords := bb.TileCoords()
		results := fetcher.GetMulti(coords, 4)

		type entry struct {
			name string
			data []byte
		}
		var entries []entry
		for _, r := range results {
			if r.Err != nil {
				if !errors.Is(r.Err, proxy.ErrNotFound) {
					log.Printf("error fetching tile %d/%d: %v", r.Coord[0], r.Coord[1], r.Err)
				}
				continue
			}
			name := fmt.Sprintf("LoD2_32_%d_%d_1_ni.gml", r.Coord[0], r.Coord[1])
			entries = append(entries, entry{name, r.Data})
		}

		if len(entries) == 0 {
			http.Error(w, "no tiles found in bbox", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename=\"lod2_tiles.zip\"")

		zw := zip.NewWriter(w)
		for _, e := range entries {
			fw, err := zw.Create(e.name)
			if err != nil {
				log.Printf("zip create %s: %v", e.name, err)
				return
			}
			if _, err := fw.Write(e.data); err != nil {
				log.Printf("zip write %s: %v", e.name, err)
				return
			}
		}
		if err := zw.Close(); err != nil {
			log.Printf("zip close: %v", err)
		}
	}
}
