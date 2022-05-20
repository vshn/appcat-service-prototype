package main

import (
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
)

/*
HOW TO RUN

# setup
make prometheus-setup
export KUBECONFIG=${PWD}/.kind/kind-kubeconfig-v1.23.0 CGO_ENABLED=0
kubectl apply -f servicemonitor.yaml
helm -n default upgrade --install pushgateway prometheus-community/prometheus-pushgateway \
	--values prometheus/pushgateway.yaml
kubectl -n prometheus-system edit servicemonitor pushgateway-prometheus-pushgateway # set path=/pushgateway/metrics

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
var stableCounter *TimestampedCounter
var pushingCounter = promauto.NewCounter(prometheus.CounterOpts{
	Name: "test_push_total",
	Help: "Test a counter with pushgateway",
})

var promHandler = promhttp.Handler()
var pusher *push.Pusher

var decreasingDelta float64 = 100000

var table = []float64{100, 120, 0, 140, 200, 150, 0, 10, 110, 80}
var tableIndex = 0
var stableValue = 100.0
var stableTimestamp = time.Now().UTC().Add(time.Minute * -1).Truncate(time.Minute)

func init() {
	setupMetrics()
}

func main() {
	log.Printf("starting exporter")
	http.HandleFunc("/metrics", scrapeHandler())
	go func() {
		for true {
			time.Sleep(10 * time.Second)
			pushToPushgateway()
		}
	}()
	err := http.ListenAndServe(":9090", nil)
	log.Fatal(err)
}

func pushToPushgateway() {
	pushingCounter.Add(1)
	err := pusher.Push()
	if err != nil {
		log.Println(err)
	} else {
		log.Println("pushed metrics")
	}
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

	// this "stable" counter allows us to return an aggregated value that is from the past.
	// Example: suppose we get the average storage capacity used only for a full day, the exporter would not be able to provide a snapshot value,
	// but only the value that covers the full day of "yesterday". We need to add a timestamp (preferably at midnight at the end) so that Prometheus knows
	// it's a fixed value from the past and can reflect that in the scrapes.
	newStable := time.Now().UTC().Truncate(time.Minute)
	if newStable.After(stableTimestamp) {
		// only every new full minute we add to the counter.
		// we can only increase a counter's value if the timestamp changes with it, otherwise Prometheus server drops it with an error like
		//  msg="Error on ingesting samples with different value but same timestamp" num_dropped=1
		stableCounter.AddWithTimestamp(stableValue, newStable)
		stableTimestamp = newStable
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
	stableCounter = NewTimestampedCounter(prometheus.Opts{
		Help: "Custom test metric to test counters and resets. It changes once only every new hour, and the timestamp is at the beginning of the new hour",
		Name: "test_stable_total",
	})

	collector.AddMetric(decreasingDeltaCounter.TimestampedMetric)
	collector.AddMetric(addingCounter.TimestampedMetric)
	collector.AddMetric(tableCounter.TimestampedMetric)
	collector.AddMetric(stableCounter.TimestampedMetric)

	prometheus.MustRegister(collector)

	pusher = push.New("http://pushgateway-prometheus-pushgateway:9091/pushgateway/", "test-job").
		Collector(pushingCounter).Grouping("region", "rma")
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
