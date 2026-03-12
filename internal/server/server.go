// Package server provides the HTTP handlers for the CityGML tile proxy.
package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/meko-tech/lgln-citygml-proxy/internal/proxy"
)

// New returns an http.Handler with all routes registered.
func New(fetcher *proxy.Fetcher) http.Handler {
	mux := http.NewServeMux()
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
