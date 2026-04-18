package main

import (
	"os"

	"github.com/cwbudde/lgln-citygml-proxy/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
