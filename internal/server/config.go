package server

import (
	"github.com/charmbracelet/log"

	"github.com/spf13/viper"
)

type Config struct {
	BGP BGP `mapstructure:"bgp"`
}

type BGP struct {
	ASN             uint32           `mapstructure:"asn"`
	RouterID        string           `mapstructure:"router_id"`
	ListenPort      int32            `mapstructure:"listen_port"`
	UpstreamPeers   []UpstreamPeer   `mapstructure:"upstream_peers"`
	DownstreamPeers []DownstreamPeer `mapstructure:"downstream_peers"`
}

type UpstreamPeer struct {
	Address string `mapstructure:"address"`
	ASN     int    `mapstructure:"asn"`
}

type DownstreamPeer struct {
	Address string `mapstructure:"address"`
	ASN     int    `mapstructure:"asn"`
	Weight  int    `mapstructure:"weight"`
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
