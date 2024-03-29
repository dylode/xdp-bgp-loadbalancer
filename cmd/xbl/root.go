package xbl

import (
	"dylode.nl/xdp-bgp-loadbalancer/cmd/xbl/reqcounter"
	"dylode.nl/xdp-bgp-loadbalancer/cmd/xbl/reqgenerator"
	"dylode.nl/xdp-bgp-loadbalancer/cmd/xbl/server"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.ExactArgs(1),
		Use:   "xbl",
		Short: "xbl is a loadbalancer used BGP as control plane and XDP as data plane",
	}

	cmd.AddCommand(server.NewCommand())
	cmd.AddCommand(reqcounter.NewCommand())
	cmd.AddCommand(reqgenerator.NewCommand())

	return cmd
}
