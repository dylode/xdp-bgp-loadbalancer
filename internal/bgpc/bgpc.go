package bgpc

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"dylode.nl/xdp-bgp-loadbalancer/pkg/graceshut"
	"github.com/charmbracelet/log"
	"github.com/osrg/gobgp/v3/pkg/server"
	"google.golang.org/protobuf/proto"

	api "github.com/osrg/gobgp/v3/api"
)

type routeWeight float64

type rib map[netip.Prefix]map[netip.Addr]routeWeight

type bgpc struct {
	config Config
	mut    sync.RWMutex

	server *server.BgpServer
	gshut  *graceshut.GraceShut

	upstreamPeers   []*api.Peer
	downstreamPeers []*api.Peer

	rib rib
}

func New(config Config) *bgpc {
	return &bgpc{
		config: config,

		server: server.NewBgpServer(server.LoggerOption(GoBGPLogger{})),
		gshut:  graceshut.New(),

		upstreamPeers:   make([]*api.Peer, len(config.UpstreamPeers)),
		downstreamPeers: make([]*api.Peer, len(config.DownstreamPeers)),

		rib: make(rib),
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
		if err := bc.update(ctx); err != nil {
			errc <- err
		}
	}()

	// wait for exit signal
	select {
	case <-bc.gshut.WaitForClose():
	case <-ctx.Done():
	case cErr := <-errc:
		err = cErr
	}

	// clean up
	log.Debug("closing bgp server controller")
	cancel()

	if err := bc.stopBGP(ctx); err != nil {
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
	for _, downStreamPeer := range bc.config.DownstreamPeers {
		peer := &api.Peer{
			Conf: &api.PeerConf{
				NeighborAddress: downStreamPeer.Address,
				PeerAsn:         bc.config.ASN,
			},
		}

		if err := bc.server.AddPeer(ctx, &api.AddPeerRequest{Peer: peer}); err != nil {
			return errors.Join(errors.New("could not add peer"), err)
		}

		bc.downstreamPeers = append(bc.downstreamPeers, peer)
	}

	for _, upStreamPeer := range bc.config.UpstreamPeers {
		peer := &api.Peer{
			Conf: &api.PeerConf{
				NeighborAddress: upStreamPeer.Address,
				PeerAsn:         upStreamPeer.ASN,
			},
		}

		if err := bc.server.AddPeer(ctx, &api.AddPeerRequest{Peer: peer}); err != nil {
			return errors.Join(errors.New("could not add peer"), err)
		}

		bc.upstreamPeers = append(bc.upstreamPeers, peer)
	}

	return nil
}

func (bc *bgpc) updateRIB(ctx context.Context) error {
	bc.mut.Lock()
	defer bc.mut.Unlock()

	rib := make(rib)
	localPrefAttr := &api.LocalPrefAttribute{}

	allowedPrefixes := make([]netip.Prefix, len(bc.config.AllowedPrefixes))
	for _, allowedPrefix := range bc.config.AllowedPrefixes {
		prefix, err := netip.ParsePrefix(allowedPrefix)
		if err != nil {
			return err
		}

		allowedPrefixes = append(allowedPrefixes, prefix)
	}

	err := bc.server.ListPath(ctx, &api.ListPathRequest{
		Family: &api.Family{
			Afi:  api.Family_AFI_IP,
			Safi: api.Family_SAFI_UNICAST,
		},
		TableType: api.TableType_GLOBAL,
	}, func(d *api.Destination) {
		prefix, err := netip.ParsePrefix(d.GetPrefix())
		if err != nil {
			log.Warn("could not parse prefix", "prefix", d.GetPrefix(), "err", err)
			return
		}

		allowed := false
		for _, allowedPrefix := range allowedPrefixes {
			if prefix.Bits() >= allowedPrefix.Bits() && allowedPrefix.Contains(prefix.Addr()) {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Debug("ignored not allowed prefix", "prefix", prefix.String())
			return
		}

		var totalWeight routeWeight
		rib[prefix] = make(map[netip.Addr]routeWeight)

		for _, path := range d.GetPaths() {
			for _, attr := range path.GetPattrs() {
				if !attr.MessageIs(localPrefAttr) {
					continue
				}

				if err := proto.Unmarshal(attr.Value, localPrefAttr); err != nil {
					continue
				}

				break
			}

			nextHop, err := netip.ParseAddr(path.GetNeighborIp())
			if err != nil {
				log.Warn("could not parse ip", "ip", path.GetNeighborIp(), "err", err)
				continue
			}

			totalWeight += routeWeight(localPrefAttr.GetLocalPref())
			rib[prefix][nextHop] = routeWeight(localPrefAttr.GetLocalPref())
		}

		for nextHop, weight := range rib[prefix] {
			rib[prefix][nextHop] = (1 / totalWeight) * weight
		}
	})
	if err != nil {
		return errors.Join(errors.New("failed updating rib"), err)
	}

	bc.rib = rib

	for prefix, nexthops := range bc.rib {
		for nexthop, weight := range nexthops {
			fmt.Printf("%s via %s [weight: %f]\n", prefix, nexthop, weight)
		}
	}

	return nil
}

func (bc *bgpc) update(ctx context.Context) error {
	run := true
	lastRun := time.Now().Add(-bc.config.UpdateInterval * 2)
	var err error

LOOP:
	for run {
		select {
		case <-ctx.Done():
			run = false
			break LOOP
		default:
			time.Sleep(time.Second)
		}

		if time.Since(lastRun) < bc.config.UpdateInterval {
			continue
		}

		err = bc.updateRIB(ctx)
		if err != nil {
			return err
		}

		lastRun = time.Now()
	}

	return nil
}

func (bc *bgpc) Close() {
	defer bc.gshut.WaitForDone()
	bc.gshut.Close()
}
