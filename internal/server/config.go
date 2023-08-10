package server

import (
	"dylode.nl/xdp-bgp-loadbalancer/internal/bgpc"
	"dylode.nl/xdp-bgp-loadbalancer/internal/xdpc"
	"github.com/charmbracelet/log"

	"github.com/spf13/viper"
)

const errorChanSize = 255

type Config struct {
	BGPC bgpc.Config `mapstructure:"bgp"`
	XDPC xdpc.Config `mapstructure:"xdp"`
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
