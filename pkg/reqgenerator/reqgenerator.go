package reqgenerator

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

func RunWithConfigFile(configFilePath string) error {
	config := ParseConfig(configFilePath)
	return Run(config)
}

func Run(config Config) error {
	transport := &http.Transport{
		MaxIdleConns:        0,
		MaxIdleConnsPerHost: 0,
		IdleConnTimeout:     0,
	}

	client := &http.Client{
		Transport: transport,
	}

	for {
		randomURL := config.URLs[rand.Intn(len(config.URLs))]

		resp, err := client.Get(randomURL)
		if err != nil {
			fmt.Println("Error:", err)
		} else {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("Response from %s: body=%s\n", randomURL, string(body))
		}

		resp.Body.Close()
		client.CloseIdleConnections()
		time.Sleep(config.Interval)
	}
}
