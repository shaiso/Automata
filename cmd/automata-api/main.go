package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	startTime = time.Now()
	reqTotal  = promauto.NewCounter(prometheus.CounterOpts{
		Name: "automata_api_http_requests_total",
		Help: "Total HTTP requests handled by automata_api",
	})
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		reqTotal.Inc()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ok %s", time.Since(startTime))
	})

	mux.Handle("/metrics", promhttp.Handler())

	addr := ":8080"
	if v := os.Getenv("API_PORT"); v != "" {
		addr = ":" + v
	}
	log.Printf("[api] listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
