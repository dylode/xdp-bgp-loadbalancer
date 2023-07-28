package reqcounter

import (
	"dylode.nl/xdp-bgp-loadbalancer/pkg/reqcounter"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reqcounter",
		Short: "start reqcounter",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			reqcounter.Run()
		},
	}

	return cmd
}
