package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:           "backend",
	Short:         "Busyness sensing API",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}
