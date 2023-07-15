package server

import (
	"dylode.nl/xdp-bgp-loadbalancer/internal/bgpc"
	"dylode.nl/xdp-bgp-loadbalancer/pkg/graceshut"
)

func RunWithConfigFile(configFilePath string) error {
	config := ParseConfig(configFilePath)
	return Run(config)
}

func Run(config Config) error {
	ctx, stop := graceshut.CreateContext()
	defer stop()

	bgpController := bgpc.New(ctx, bgpc.Config{
		ASN:        config.BGP.ASN,
		RouterID:   config.BGP.RouterID,
		ListenPort: config.BGP.ListenPort,
	})
	go bgpController.Run()

	<-ctx.Done()

	bgpController.Close()

	return nil
}
