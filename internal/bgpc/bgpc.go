package bgpc

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"dylode.nl/xdp-bgp-loadbalancer/pkg/graceshut"
	"github.com/charmbracelet/log"
	"github.com/osrg/gobgp/v3/pkg/apiutil"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
	"github.com/osrg/gobgp/v3/pkg/server"
	"google.golang.org/protobuf/proto"

	api "github.com/osrg/gobgp/v3/api"
)

type vrf string

const (
	DOWNSTREAM_VRF vrf = "downstream"
	UPSTREAM_VRF   vrf = "upstream"
)

type weight int

type rib map[netip.Prefix]map[netip.Addr]weight

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

	err = bc.configureVRFs(ctx)
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

func (bc *bgpc) configureVRFs(ctx context.Context) error {
	for index, vrfName := range []vrf{UPSTREAM_VRF, DOWNSTREAM_VRF} {
		var rd bgp.RouteDistinguisherInterface
		rd, err := bgp.ParseRouteDistinguisher(fmt.Sprintf("%d:%d", bc.config.ASN, index*100))
		if err != nil {
			return err
		}

		v, err := apiutil.MarshalRD(rd)
		if err != nil {
			return err
		}

		rt, err := bgp.ParseRouteTarget(fmt.Sprintf("%d:%d", bc.config.ASN, index*100))
		if err != nil {
			return err
		}

		rts, err := apiutil.MarshalRTs([]bgp.ExtendedCommunityInterface{
			rt,
		})
		if err != nil {
			return err
		}

		err = bc.server.AddVrf(ctx, &api.AddVrfRequest{
			Vrf: &api.Vrf{
				Name:     string(vrfName),
				Rd:       v,
				ImportRt: rts,
				ExportRt: rts,
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (bc *bgpc) addPeers(ctx context.Context) error {
	for _, downStreamPeer := range bc.config.DownstreamPeers {
		peer := &api.Peer{
			Conf: &api.PeerConf{
				NeighborAddress: downStreamPeer.Address,
				PeerAsn:         bc.config.ASN,
				Vrf:             string(DOWNSTREAM_VRF),
			},
		}

		if err := bc.server.AddPeer(ctx, &api.AddPeerRequest{Peer: peer}); err != nil {
			return errors.Join(errors.New("could not add peer"), err)
		}

		bc.downstreamPeers = append(bc.downstreamPeers, peer)
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
		TableType: api.TableType_VRF,
		Name:      string(DOWNSTREAM_VRF),
	}, func(d *api.Destination) {
		prefix, err := netip.ParsePrefix(strings.Split(d.GetPrefix(), ":")[2])
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

		rib[prefix] = make(map[netip.Addr]weight)

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

			rib[prefix][nextHop] = weight(localPrefAttr.GetLocalPref())
		}
	})
	if err != nil {
		return errors.Join(errors.New("failed updating rib"), err)
	}

	bc.rib = rib

	//fmt.Println(rib)

	for prefix, nexthops := range bc.rib {
		for nexthop, weight := range nexthops {
			fmt.Printf("%s via %s [weight: %d]\n", prefix, nexthop, weight)

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
