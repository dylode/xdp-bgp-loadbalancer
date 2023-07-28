package reqcounter

import (
	"fmt"

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

	mux.HandleFunc("/increase", func(w http.ResponseWriter, r *http.Request) {
		requestCount.Inc()
		fmt.Printf("Request from %s\n", r.RemoteAddr)
		fmt.Fprintf(w, "Hi %s", r.RemoteAddr)
	})

	mux.Handle("/metrics", promhttp.Handler())

	fmt.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}
