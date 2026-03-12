package main

import (
	"os"

	"github.com/meko-tech/lgln-citygml-proxy/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
