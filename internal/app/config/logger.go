package config

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type LoggerConfig struct {
	Level zerolog.Level
}

func parseLoggerConfig(v *viper.Viper) (LoggerConfig, error) {
	level, err := zerolog.ParseLevel(v.GetString("logger.level"))
	if err != nil {
		return LoggerConfig{}, fmt.Errorf("logger.level: %w", err)
	}
	return LoggerConfig{Level: level}, nil
}
