package reqcounter

import (
	"fmt"
	"os"

	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "total_requests",
		Help: "Total number of requests received.",
	})
)

func Run() {
	prometheus.MustRegister(requestCount)

	mux := http.NewServeMux()

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	mux.HandleFunc("/increase", func(w http.ResponseWriter, r *http.Request) {
		requestCount.Inc()
		fmt.Printf("Request from %s\n", r.RemoteAddr)
		fmt.Fprintf(w, "Hi %s from %s", r.RemoteAddr, hostname)
	})

	mux.Handle("/metrics", promhttp.Handler())

	fmt.Println("Server started on :8080")
	err = http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}
