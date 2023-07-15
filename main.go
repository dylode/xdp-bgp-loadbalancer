package main

import (
	"os"

	"dylode.nl/xdp-bgp-loadbalancer/cmd/xgl"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := xgl.NewCommand()
	cmd.SetArgs(args)
	return cmd.Execute()
}
