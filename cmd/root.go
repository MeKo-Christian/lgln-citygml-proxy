package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "lgln-citygml-proxy",
	Short: "Proxy server for LGLN Niedersachsen CityGML LoD2 tiles",
}

func Execute() error {
	return rootCmd.Execute()
}
