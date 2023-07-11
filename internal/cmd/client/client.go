package client

import (
	"dylode.nl/xdp-bgp-loadbalancer/pkg/xgl/client"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	var configFilePath string

	cmd := &cobra.Command{
		Use:   "client",
		Short: "start xbl client",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.RunWithConfigFile(configFilePath)
		},
	}

	cmd.Flags().StringVar(&configFilePath, "config", "client.yaml", "path to client config file")

	return cmd
}
