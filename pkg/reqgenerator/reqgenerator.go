package reqgenerator

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "total_requests",
		Help: "Total number of requests sent.",
	})
)

func RunWithConfigFile(configFilePath string) error {
	config := ParseConfig(configFilePath)
	return Run(config)
}

func Run(config Config) error {
	prometheus.MustRegister(requestCount)

	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.Handler())

	go func() {
		fmt.Println("Server started on :8080")
		err := http.ListenAndServe(":8080", mux)
		if err != nil {
			panic(err)
		}
	}()

	transport := &http.Transport{
		MaxIdleConns:        0,
		MaxIdleConnsPerHost: 0,
		IdleConnTimeout:     0,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   3 * time.Second,
	}

	for {
		time.Sleep(config.Interval)

		randomURL := config.URLs[rand.Intn(len(config.URLs))]

		resp, err := client.Get(randomURL)
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		requestCount.Inc()
		fmt.Printf("Response from %s: body=%s\n", randomURL, string(body))
		resp.Body.Close()
		client.CloseIdleConnections()
	}
}
