package cmd

import (
	"dylode.nl/xdp-bgp-loadbalancer/internal/cmd/client"
	"dylode.nl/xdp-bgp-loadbalancer/internal/cmd/server"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.ExactArgs(1),
		Use:   "xbl",
		Short: "xbl is a loadbalancer used BGP as control plane and XDP as data plane",
	}

	cmd.AddCommand(server.NewCommand())
	cmd.AddCommand(client.NewCommand())

	return cmd
}