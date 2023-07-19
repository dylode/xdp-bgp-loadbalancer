package bgpc

import "time"

const errorChanSize = 255

type Config struct {
	ASN             uint32                 `mapstructure:"asn"`
	RouterID        string                 `mapstructure:"router_id"`
	ListenPort      int32                  `mapstructure:"listen_port"`
	UpdateInterval  time.Duration          `mapstructure:"update_interval"`
	AllowedPrefixes []string               `mapstructure:"allowed_prefixes"`
	UpstreamPeers   []UpstreamPeerConfig   `mapstructure:"upstream_peers"`
	DownstreamPeers []DownstreamPeerConfig `mapstructure:"downstream_peers"`
}

type UpstreamPeerConfig struct {
	Address string `mapstructure:"address"`
	ASN     uint32 `mapstructure:"asn"`
}

type DownstreamPeerConfig struct {
	Address string `mapstructure:"address"`
}
