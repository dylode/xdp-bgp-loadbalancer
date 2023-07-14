package bgpc

import (
	"context"
	"errors"

	"dylode.nl/xdp-bgp-loadbalancer/pkg/graceclose"
	"github.com/charmbracelet/log"
	"github.com/osrg/gobgp/v3/pkg/server"

	api "github.com/osrg/gobgp/v3/api"
)

type Config struct {
	ASN        uint32
	RouterID   string
	ListenPort int32
}

type bgpc struct {
	ctx    context.Context
	config Config

	server *server.BgpServer
	gc     *graceclose.GraceClose
}

func New(ctx context.Context, config Config) *bgpc {
	return &bgpc{
		ctx:    ctx,
		config: config,

		server: server.NewBgpServer(),
		gc:     graceclose.New(),
	}
}

func (bc *bgpc) Run() error {
	defer bc.gc.Done()

	log.Debug("starting bgp server controller")
	go bc.server.Serve()

	err := bc.server.StartBgp(bc.ctx, &api.StartBgpRequest{
		Global: &api.Global{
			Asn:        bc.config.ASN,
			RouterId:   bc.config.RouterID,
			ListenPort: bc.config.ListenPort,
		},
	})
	if err != nil {
		return errors.Join(errors.New("could not start bgp server"), err)
	}

	log.Info("bgp server running")

	bc.gc.WaitForClose()

	log.Debug("closing bgp server controller")
	
	err = bc.server.StopBgp(context.Background(), &api.StopBgpRequest{})
	if err != nil {
		return errors.Join(errors.New("could not stop bgp server"))
	}

	log.Info("bgp server stopped")

	return nil
}

func (bc *bgpc) Close() {
	defer bc.gc.WaitForDone()
	bc.gc.Close()
}
