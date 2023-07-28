package reqgenerator

import (
	"dylode.nl/xdp-bgp-loadbalancer/pkg/reqgenerator"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	var configFilePath string

	cmd := &cobra.Command{
		Use:   "reqgenerator",
		Short: "start reqgenerator",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			err := reqgenerator.RunWithConfigFile(configFilePath)
			if err != nil {
				log.Error("exited due to error", "err", err)
			}
		},
	}

	cmd.Flags().StringVar(&configFilePath, "config", "reqgenerator-config.yaml", "path to reqgenerator config file")

	return cmd
}
