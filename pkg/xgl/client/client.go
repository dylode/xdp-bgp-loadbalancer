package client

import "fmt"

func RunWithConfigFile(configFilePath string) error {
	config := ParseConfig(configFilePath)
	return Run(config)
}

func Run(config Config) error {
	fmt.Println(config.Servers)
	return nil
}
