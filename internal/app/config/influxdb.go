package config

import "github.com/spf13/viper"

type InfluxDBConfig struct {
	Host     string `mapstructure:"host"`
	Database string `mapstructure:"database"`
	Token    string `mapstructure:"token"`
}

func parseInfluxDBConfig(v *viper.Viper) (InfluxDBConfig, error) {
	var c InfluxDBConfig
	if err := v.UnmarshalKey("influxdb", &c); err != nil {
		return InfluxDBConfig{}, err
	}
	return c, nil
}
