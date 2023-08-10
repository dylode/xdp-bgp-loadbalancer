package xdpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"

	"dylode.nl/xdp-bgp-loadbalancer/pkg/graceshut"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc $BPF_CLANG -cflags $BPF_CFLAGS bpf xdp.c -- -I./headers

type xdpc struct {
	config Config
	gshut  *graceshut.GraceShut
}

func New(config Config) *xdpc {
	return &xdpc{
		config: config,
		gshut:  graceshut.New(),
	}
}

func (xdpc xdpc) Run(ctx context.Context) error {
	defer xdpc.gshut.Done()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.Debug("starting xdp controller")

	// Look up the network interface by name.
	ifaceName := xdpc.config.InterfaceName

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return errors.Join(errors.New("lookup network interface failed"), err)
	}

	// Load pre-compiled programs into the kernel.
	objs := bpfObjects{}
	if err := loadBpfObjects(&objs, nil); err != nil {
		return errors.Join(errors.New("loading objects failed"), err)
	}

	// Attach the program.
	l, err := link.AttachXDP(link.XDPOptions{
		Program:   objs.XdpProgFunc,
		Interface: iface.Index,
	})
	if err != nil {
		return errors.Join(errors.New("could not attach xdp program"), err)
	}

	errc := make(chan error, errorChanSize)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := xdpc.update(ctx, objs); err != nil {
			errc <- err
		}
	}()

	// wait for exit signal
	select {
	case <-xdpc.gshut.WaitForClose():
	case <-ctx.Done():
	case cErr := <-errc:
		err = cErr
	}

	log.Debug("closing xdp controller")
	cancel()

	objs.Close()
	l.Close()

	wg.Wait()

	// process errors
	close(errc)
	for cErr := range errc {
		err = errors.Join(err, cErr)
	}

	return err
}

func (xdpc xdpc) update(ctx context.Context, objs bpfObjects) error {
	// Print the contents of the BPF hash map (source IP address -> packet count).
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
LOOP:
	for range ticker.C {
		select {
		case <-ctx.Done():
			ticker.Stop()
			break LOOP
		default:
		}

		s, err := formatMapContents(objs.XdpStatsMap)
		if err != nil {
			log.Printf("Error reading map: %s", err)
			continue
		}
		log.Printf("Map contents:\n%s", s)
	}

	return nil
}

func formatMapContents(m *ebpf.Map) (string, error) {
	var (
		sb  strings.Builder
		key []byte
		val uint32
	)
	iter := m.Iterate()
	for iter.Next(&key, &val) {
		sourceIP := net.IP(key) // IPv4 source address in network byte order.
		packetCount := val
		sb.WriteString(fmt.Sprintf("\t%s => %d\n", sourceIP, packetCount))
	}
	return sb.String(), iter.Err()
}
