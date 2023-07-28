package reqgenerator

import (
	"log"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Interval time.Duration `mapstructure:"interval"`
	URLs     []string      `mapstructure:"urls"`
}

func ParseConfig(configFilePath string) Config {
	viper.SetConfigFile(configFilePath)

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("could not read config file", "err", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatal("could not parse config", "err", err)
	}

	return config
}
