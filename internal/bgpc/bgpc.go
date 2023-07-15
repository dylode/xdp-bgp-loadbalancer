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

	var wg sync.WaitGroup

	wg.Add(1)
	go bc.listPeers(ctx, &wg)

	bc.gshut.WaitForClose()

	log.Debug("closing bgp server controller")
	cancel()

	err = bc.stopBGP(context.Background())
	if err != nil {
		return err
	}

	wg.Wait()

	return nil
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
		return errors.Join(errors.New("could not stop bgp server"))
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

func (bc *bgpc) listPeers(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

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
			log.Error("could not list path", "err", err)
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
}

func (bc *bgpc) Close() {
	defer bc.gshut.WaitForDone()
	bc.gshut.Close()
}
