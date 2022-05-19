package main

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

/*
HOW TO RUN

# setup
make prometheus-setup
export KUBECONFIG=${PWD}/.kind/kind-kubeconfig-v1.23.0 CGO_ENABLED=0
kubectl apply -f servicemonitor.yaml

# deploy image
go build -o prom main.go
docker build -t local.dev/vshn/appcat-service-prototype:prom .
kind load docker-image --name appcat-service-prototype local.dev/vshn/appcat-service-prototype:prom

# (re)start pod
kubectl delete pod prometheus-test-counter --ignore-not-found
kubectl run --image local.dev/vshn/appcat-service-prototype:prom --port 9090 prometheus-test-counter --labels app=prom-test

*/

var addingCounter = promauto.NewCounter(prometheus.CounterOpts{
	Help: "Custom test metric to test counters and resets. Increases counter by a constant number on each scrape",
	Name: "test_constant_delta_total",
})

var decreasingDeltaCounter = promauto.NewCounter(prometheus.CounterOpts{
	Help: "Custom test metric to test counters and resets. Starts with a high value and adds less and less over multiple scrapes",
	Name: "test_decreasing_delta_total",
})

var tableCounter = promauto.NewCounter(prometheus.CounterOpts{
	Help: "Custom test metric to test counters and resets. Picks a value from a table and adds to counter, next scrape picks another row. At the end it begins anew.",
	Name: "test_table_total",
})

var promHandler = promhttp.Handler()

var decreasingDelta float64 = 100000

var table = []float64{100, 120, 0, 140, 200, 150, 0, 10, 110, 80}
var tableIndex = 0

func main() {
	log.Printf("starting exporter")
	http.HandleFunc("/metrics", scrapeHandler())
	err := http.ListenAndServe(":9090", nil)
	log.Fatal(err)
}

func recordMetrics() {
	decreasingDeltaCounter.Add(decreasingDelta)
	decreasingDelta /= 2
	addingCounter.Add(10)
	tableCounter.Add(table[tableIndex])
	if tableIndex >= len(table)-1 {
		tableIndex = 0
	} else {
		tableIndex += 1
	}
}

func scrapeHandler() func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		log.Printf("scrape from %s", request.RemoteAddr)
		recordMetrics()
		promHandler.ServeHTTP(writer, request)
	}
}
