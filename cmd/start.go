package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jouleion/busyness-sensing/backend/internal/app/config"
	"github.com/jouleion/busyness-sensing/backend/internal/app/log"
	"github.com/jouleion/busyness-sensing/backend/internal/driver/http"
)

var configPath string

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the API server",
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := config.Init(configPath); err != nil {
			return fmt.Errorf("load configuration: %w", err)
		}

		log.Init()
		return http.NewServer().Start()
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVarP(
		&configPath,
		"config",
		"c",
		"configs/configs.webserver.yml",
		"configuration file path",
	)
}
