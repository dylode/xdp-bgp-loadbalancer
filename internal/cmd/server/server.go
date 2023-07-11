package server

import (
	"dylode.nl/xdp-bgp-loadbalancer/internal/server"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	var configFilePath string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "start xbl server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return server.RunWithConfigFile(configFilePath)
		},
	}

	cmd.Flags().StringVar(&configFilePath, "config", "server.yaml", "path to server config file")

	return cmd
}
