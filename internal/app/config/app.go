package config

import "github.com/spf13/viper"

type AppConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"env"`
}

func parseAppConfig(v *viper.Viper) (AppConfig, error) {
	var c AppConfig
	if err := v.UnmarshalKey("app", &c); err != nil {
		return AppConfig{}, err
	}
	return c, nil
}
