package cmd

import (
	"fmt"
	"log"
	"net/http"

	"github.com/meko-tech/lgln-citygml-proxy/internal/proxy"
	"github.com/meko-tech/lgln-citygml-proxy/internal/server"
	"github.com/spf13/cobra"
)

var (
	port     int
	cacheDir string
	stacURL  string
)

func init() {
	serveCmd.Flags().IntVarP(&port, "port", "p", 8080, "port to listen on")
	serveCmd.Flags().StringVarP(&cacheDir, "cache-dir", "c", "./cache", "directory for cached tiles")
	serveCmd.Flags().StringVar(&stacURL, "stac-url", "",
		"STAC API base URL for tile discovery and cache freshness (default: disabled, use hardcoded grid)")
	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the CityGML tile proxy server",
	RunE: func(_ *cobra.Command, _ []string) error {
		var fetcher *proxy.Fetcher
		if stacURL != "" {
			fetcher = proxy.NewWithSTAC(cacheDir, stacURL)
			log.Printf("STAC tile discovery enabled: %s", stacURL)
		} else {
			fetcher = proxy.New(cacheDir)
		}

		mux := server.New(fetcher)

		addr := fmt.Sprintf(":%d", port)
		log.Printf("listening on %s (cache: %s)", addr, cacheDir)

		return http.ListenAndServe(addr, mux)
	},
}
