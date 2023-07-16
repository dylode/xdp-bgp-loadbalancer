package bgpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"dylode.nl/xdp-bgp-loadbalancer/pkg/graceshut"
	"github.com/charmbracelet/log"
	"github.com/osrg/gobgp/v3/pkg/server"

	api "github.com/osrg/gobgp/v3/api"
)

type bgpc struct {
	config Config

	server *server.BgpServer
	gshut  *graceshut.GraceShut

	upstreamPeers []*api.Peer
}

func New(config Config) *bgpc {
	return &bgpc{
		config: config,

		server: server.NewBgpServer(server.LoggerOption(GoBGPLogger{})),
		gshut:  graceshut.New(),
	}
}

func (bc *bgpc) Run(ctx context.Context) error {
	defer bc.gshut.Done()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// initialization
	log.Debug("starting bgp server controller")
	go bc.server.Serve()

	err := bc.startBGP(ctx)
	if err != nil {
		return err
	}

	err = bc.addPeers(ctx)
	if err != nil {
		return err
	}

	// run state
	errc := make(chan error, errorChanSize)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := bc.listPeers(ctx); err != nil {
			errc <- err
		}
	}()

	// wait for exit signal
	select {
	case cErr := <-errc:
		err = cErr
	case <-bc.gshut.WaitForClose():
	case <-ctx.Done():
	}

	// clean up
	log.Debug("closing bgp server controller")
	cancel()

	if err := bc.stopBGP(context.Background()); err != nil {
		errc <- err
	}

	wg.Wait()

	// process errors
	close(errc)
	for cErr := range errc {
		err = errors.Join(err, cErr)
	}

	return err
}

func (bc *bgpc) startBGP(ctx context.Context) error {
	err := bc.server.StartBgp(ctx, &api.StartBgpRequest{
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
	return nil
}

func (bc *bgpc) stopBGP(ctx context.Context) error {
	if err := bc.server.StopBgp(ctx, &api.StopBgpRequest{}); err != nil {
		return errors.Join(errors.New("could not stop bgp server"), err)
	}

	log.Info("bgp server stopped")
	return nil
}

func (bc *bgpc) addPeers(ctx context.Context) error {
	for _, upstreamPeer := range bc.config.UpstreamPeers {
		peer := &api.Peer{
			Conf: &api.PeerConf{
				NeighborAddress: upstreamPeer.Address,
				PeerAsn:         uint32(upstreamPeer.ASN),
			},
		}

		if err := bc.server.AddPeer(ctx, &api.AddPeerRequest{Peer: peer}); err != nil {
			return errors.Join(errors.New("could not add peer"), err)
		}

		bc.upstreamPeers = append(bc.upstreamPeers, peer)
	}

	return nil
}

func (bc *bgpc) listPeers(ctx context.Context) error {
	run := true
	for run {
		err := bc.server.ListPath(ctx, &api.ListPathRequest{
			Family: &api.Family{
				Afi:  api.Family_AFI_IP,
				Safi: api.Family_SAFI_UNICAST,
			},
		}, func(d *api.Destination) {
			fmt.Println(d.Prefix, len(d.Paths))

			for _, path := range d.Paths {
				fmt.Printf("%s \n", path.NeighborIp)

				for _, attr := range path.Pattrs {
					fmt.Println(attr)
				}
			}
		})
		if err != nil {
			return errors.Join(errors.New("could not list path"), err)
		}

		//bc.server.ListPeer(ctx, &api.ListPeerRequest{}, func(p *api.Peer) {
		//	fmt.Println(p)
		//})

		time.Sleep(1 * time.Second)

		select {
		case <-ctx.Done():
			run = false
		default:
		}

	}

	return nil
}

func (bc *bgpc) Close() {
	defer bc.gshut.WaitForDone()
	bc.gshut.Close()
}
