package main

import (
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

var decreasingDeltaCounter *TimestampedCounter
var addingCounter *TimestampedCounter
var tableCounter *TimestampedCounter

var promHandler = promhttp.Handler()

var decreasingDelta float64 = 100000

var table = []float64{100, 120, 0, 140, 200, 150, 0, 10, 110, 80}
var tableIndex = 0

func init() {
	setupMetrics()
}

func main() {
	log.Printf("starting exporter")
	http.HandleFunc("/metrics", scrapeHandler())
	err := http.ListenAndServe(":9090", nil)
	log.Fatal(err)
}

func recordMetrics() {
	decreasingDeltaCounter.AddWithTimestamp(decreasingDelta, time.Date(2022, 12, 2, 12, 54, 31, 0, time.UTC))
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

type TimestampedCounter struct {
	TimestampedMetric
	value     float64
	timestamp time.Time
}

func (c *TimestampedCounter) Add(delta float64) {
	c.AddWithTimestamp(delta, time.Now().UTC())
}

func (c *TimestampedCounter) AddWithTimestamp(delta float64, timestamp time.Time) {
	c.value += delta
	c.timestamp = timestamp
}

func setupMetrics() {
	collector := &myCollector{metrics: []TimestampedMetric{}}
	decreasingDeltaCounter = NewTimestampedCounter(prometheus.Opts{
		Help: "Custom test metric to test counters and resets. Starts with a high value and adds less and less over multiple scrapes",
		Name: "test_decreasing_delta_total",
	})
	addingCounter = NewTimestampedCounter(prometheus.Opts{
		Help: "Custom test metric to test counters and resets. Increases counter by a constant number on each scrape",
		Name: "test_constant_delta_total",
	})
	tableCounter = NewTimestampedCounter(prometheus.Opts{
		Help: "Custom test metric to test counters and resets. Picks a value from a table and adds to counter, next scrape picks another row. At the end it begins anew.",
		Name: "test_table_total",
	})

	collector.AddMetric(decreasingDeltaCounter.TimestampedMetric)
	collector.AddMetric(addingCounter.TimestampedMetric)
	collector.AddMetric(tableCounter.TimestampedMetric)

	prometheus.MustRegister(collector)
}

func NewTimestampedCounter(opts prometheus.Opts) *TimestampedCounter {
	c := &TimestampedCounter{
		value: 0,
	}
	c.TimestampedMetric = TimestampedMetric{
		Opts: opts,
		ValueGetter: func() (float64, time.Time) {
			return c.value, c.timestamp
		},
	}
	return c
}

type myCollector struct {
	metrics []TimestampedMetric
}

type TimestampedMetric struct {
	ValueGetter func() (float64, time.Time)
	Opts        prometheus.Opts
	desc        *prometheus.Desc
}

func (m *myCollector) AddMetric(metric TimestampedMetric) {
	metric.desc = prometheus.NewDesc(metric.Opts.Name, metric.Opts.Help, nil, metric.Opts.ConstLabels)
	m.metrics = append(m.metrics, metric)
}

func (m *myCollector) Describe(descs chan<- *prometheus.Desc) {
	for _, metric := range m.metrics {
		descs <- metric.desc
	}
}

func (m *myCollector) Collect(metrics chan<- prometheus.Metric) {
	for _, metric := range m.metrics {
		value, timestamp := metric.ValueGetter()
		s := prometheus.NewMetricWithTimestamp(timestamp, prometheus.MustNewConstMetric(metric.desc, prometheus.CounterValue, value))
		metrics <- s
	}
}
