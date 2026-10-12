package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	App    AppConfig
	Logger LoggerConfig
	HTTP   HTTPConfig
}

var C Config

func Init(path string) error {
	v := viper.New()
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var c Config
	var err error
	c.App, err = parseAppConfig(v)
	if err != nil {
		return fmt.Errorf("app config: %w", err)
	}
	c.Logger, err = parseLoggerConfig(v)
	if err != nil {
		return fmt.Errorf("logger config: %w", err)
	}
	c.HTTP, err = parseHTTPConfig(v)
	if err != nil {
		return fmt.Errorf("http config: %w", err)
	}

	C = c
	return nil
}
