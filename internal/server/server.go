package server

import (
	"fmt"

)

func RunWithConfigFile(configFilePath string) error {
	config := ParseConfig(configFilePath)
	return Run(config)
}

func Run(config Config) error {
	fmt.Println(config.LoadBalancers[0].Layer)
	return nil
}
