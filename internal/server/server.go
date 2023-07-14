package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"dylode.nl/xdp-bgp-loadbalancer/internal/bgpc"
)

func RunWithConfigFile(configFilePath string) error {
	config := ParseConfig(configFilePath)
	return Run(config)
}

func Run(config Config) error {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	defer stop()

	bgpController := bgpc.New(ctx, bgpc.Config{
		ASN:        config.LoadBalancers[0].BGP.ASN,
		RouterID:   config.LoadBalancers[0].BGP.RouterID,
		ListenPort: config.LoadBalancers[0].BGP.ListenPort,
	})
	go bgpController.Run()

	<-ctx.Done()

	bgpController.Close()

	return nil
}
