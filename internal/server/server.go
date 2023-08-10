package server

import (
	"errors"
	"sync"

	"dylode.nl/xdp-bgp-loadbalancer/internal/bgpc"
	"dylode.nl/xdp-bgp-loadbalancer/internal/xdp"
	"dylode.nl/xdp-bgp-loadbalancer/pkg/graceshut"
)

func RunWithConfigFile(configFilePath string) error {
	config := ParseConfig(configFilePath)
	return Run(config)
}

func Run(config Config) error {
	ctx, cancel := graceshut.CreateContext()
	defer cancel()

	errc := make(chan error, errorChanSize)
	var wg sync.WaitGroup

	bgpController := bgpc.New(config.BGPC)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := bgpController.Run(ctx); err != nil {
			errc <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := xdp.Run(config.XDP); err != nil {
			errc <- err
		}
	}()

	// wait for exit signal
	var err error
	select {
	case cErr := <-errc:
		err = cErr
		cancel()
	case <-ctx.Done():
	}

	// clean up
	bgpController.Close()
	wg.Wait()

	// process errors
	close(errc)
	if ctx.Err() == nil {
		for cErr := range errc {
			err = errors.Join(err, cErr)
		}
	}

	return err
}
