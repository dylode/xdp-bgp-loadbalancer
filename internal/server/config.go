package server

import (
	"log"

	"github.com/spf13/viper"
)

type LoadBalancerLayer int

type UpstreamMode string
type DownstreamMode string

const (
	LoadBalancerLayer3 LoadBalancerLayer = 3
	LoadBalancerLayer4 LoadBalancerLayer = 4
)

const (
	UpstreamModeBGP UpstreamMode = "BGP"
)

const (
	DownstreamModeBGP DownstreamMode = "BGP"
)

type Config struct {
	LoadBalancers []LoadBalancer `mapstructure:"loadbalancers"`
}

type LoadBalancer struct {
	Name  string            `mapstructure:"name"`
	Layer LoadBalancerLayer `mapstructure:"layer"`
}

type BGP struct {
	ASN        int    `mapstructure:"asn"`
	RouterID   string `mapstructure:"routerID"`
	ListenPort uint   `mapstructure:"listenPort"`
}

type Upstream struct {
	Mode  UpstreamMode   `mapstructure:"mode"`
	Peers []UpstreamPeer `mapstructure:"peers"`
}

type Downstream struct {
	Mode  DownstreamMode   `mapstructure:"mode"`
	Peers []DownstreamPeer `mapstructure:"peers"`
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
