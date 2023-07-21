package server

import (
	"dylode.nl/xdp-bgp-loadbalancer/internal/server"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	var configFilePath string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "start xbl server",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			err := server.RunWithConfigFile(configFilePath)
			if err != nil {
				log.Error("exited due to error", "err", err)
			}
		},
	}

	cmd.Flags().StringVar(&configFilePath, "config", "config.yaml", "path to config file")

	return cmd
}
