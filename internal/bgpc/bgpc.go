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

	rib             rib
	allowedPrefixes []netip.Prefix
}

func New(config Config) *bgpc {
	return &bgpc{
		config: config,

		server: server.NewBgpServer(server.LoggerOption(GoBGPLogger{})),
		gshut:  graceshut.New(),

		upstreamPeers:   make([]*api.Peer, len(config.UpstreamPeers)),
		downstreamPeers: make([]*api.Peer, len(config.DownstreamPeers)),

		rib:             make(rib),
		allowedPrefixes: make([]netip.Prefix, len(config.AllowedPrefixes)),
	}
}

func (bc *bgpc) Run(ctx context.Context) error {
	defer bc.gshut.Done()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// initialization
	log.Debug("starting bgp server controller")
	go bc.server.Serve()

	err := bc.setAllowedPrefixes()
	if err != nil {
		return err
	}

	err = bc.startBGP(ctx)
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

func (bc *bgpc) setAllowedPrefixes() error {
	for _, allowedPrefix := range bc.config.AllowedPrefixes {
		prefix, err := netip.ParsePrefix(allowedPrefix)
		if err != nil {
			return err
		}

		bc.allowedPrefixes = append(bc.allowedPrefixes, prefix)
	}

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

	err := bc.server.ListPath(ctx, &api.ListPathRequest{
		Family: &api.Family{
			Afi:  api.Family_AFI_IP,
			Safi: api.Family_SAFI_UNICAST,
		},
	}, func(d *api.Destination) {
		prefix, err := netip.ParsePrefix(d.GetPrefix())
		if err != nil {
			log.Warn("could not parse prefix", "prefix", d.GetPrefix(), "err", err)
			return
		}

		if !bc.isAllowedPrefix(prefix) {
			log.Debug("prefix is not allowed", "prefix", prefix.String())
			return
		}

		var totalWeight routeWeight
		rib[prefix] = make(map[netip.Addr]routeWeight)

		for _, path := range d.GetPaths() {
			localPreference, err := bc.getLocalPreference(path)
			if err != nil {
				log.Warn("could not find local preference", "uuid", path.Uuid, "err", err)
				continue
			}

			nextHop, err := bc.getNextHop(path)
			if err != nil {
				log.Warn("could not parse next hop", "err", err)
				continue
			}

			weight := routeWeight(localPreference)
			totalWeight += weight
			rib[prefix][*nextHop] = weight
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

func (bc *bgpc) getLocalPreference(path *api.Path) (uint32, error) {
	localPrefAttr := &api.LocalPrefAttribute{}

	for _, attr := range path.GetPattrs() {
		if !attr.MessageIs(localPrefAttr) {
			continue
		}

		if err := proto.Unmarshal(attr.Value, localPrefAttr); err != nil {
			return 0, errors.Join(errors.New("error during unmarshal LocalPrefAttribute"), err)
		}

		return localPrefAttr.GetLocalPref(), nil
	}

	return 0, errors.New("could not find local preference attribute in path")
}

func (bc *bgpc) getNextHop(path *api.Path) (*netip.Addr, error) {
	nextHopAttr := &api.NextHopAttribute{}

	for _, attr := range path.GetPattrs() {
		if !attr.MessageIs(nextHopAttr) {
			continue
		}

		if err := proto.Unmarshal(attr.Value, nextHopAttr); err != nil {
			return nil, errors.Join(errors.New("error during unmarshal NextHopAttribute"), err)
		}

		nextHop, err := netip.ParseAddr(nextHopAttr.GetNextHop())
		if err != nil {
			return nil, errors.Join(errors.New("could not parse next hop"), err)
		}

		return &nextHop, nil
	}

	return nil, errors.New("could not find next hop attribute in path")
}

func (bc *bgpc) isAllowedPrefix(prefix netip.Prefix) bool {
	for _, allowedPrefix := range bc.allowedPrefixes {
		if prefix.Bits() >= allowedPrefix.Bits() && allowedPrefix.Contains(prefix.Addr()) {
			return true
		}
	}

	return false
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
