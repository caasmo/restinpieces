package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// requestsTotalName is the counter the prerouter middleware increments for
// every handled request.
const requestsTotalName = "http_server_requests_total"

// Metric holds the collectors the framework's middleware and handlers share.
// New collectors become fields on this struct. It carries no registry: the
// app only needs the counters, the registry lives on the daemon path.
type Metric struct {
	RequestsTotal *prometheus.CounterVec
}

func NewMetric() *Metric {
	requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: requestsTotalName,
		Help: "Total number of HTTP requests handled by the server, labeled by status code.",
	}, []string{"code"})

	return &Metric{RequestsTotal: requestsTotal}
}
