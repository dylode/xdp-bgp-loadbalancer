package bgpc

type Config struct {
	ASN             uint32                 `mapstructure:"asn"`
	RouterID        string                 `mapstructure:"router_id"`
	ListenPort      int32                  `mapstructure:"listen_port"`
	UpstreamPeers   []UpstreamPeerConfig   `mapstructure:"upstream_peers"`
	DownstreamPeers []DownstreamPeerConfig `mapstructure:"downstream_peers"`
}

type UpstreamPeerConfig struct {
	Address string `mapstructure:"address"`
	ASN     int    `mapstructure:"asn"`
}

type DownstreamPeerConfig struct {
	Address string `mapstructure:"address"`
	ASN     int    `mapstructure:"asn"`
	Weight  int    `mapstructure:"weight"`
}
