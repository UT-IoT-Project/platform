package config

import "github.com/spf13/viper"

type HTTPConfig struct {
	Address string `mapstructure:"address"`
	Port    string `mapstructure:"port"`
}

func parseHTTPConfig(v *viper.Viper) (HTTPConfig, error) {
	var c HTTPConfig
	if err := v.UnmarshalKey("http", &c); err != nil {
		return HTTPConfig{}, err
	}
	return c, nil
}
