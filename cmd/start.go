package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jouleion/busyness-sensing/backend/internal/app/config"
	"github.com/jouleion/busyness-sensing/backend/internal/app/log"
	"github.com/jouleion/busyness-sensing/backend/internal/core/service"
	"github.com/jouleion/busyness-sensing/backend/internal/driven/database/influxdb"
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
		client, err := influxdb.Init()
		if err != nil {
			log.Logger.Fatal().Err(err).Msg("failed to initialize database")
			return err
		}
		defer client.Close()
		repository := influxdb.NewReadingsRepository(client)
		readingsService := service.NewReadingsService(repository)
		return http.NewServer(readingsService).Start()
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
