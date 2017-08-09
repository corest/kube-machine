package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	nodeGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: Prefix + "nodes_count",
			Help: "Number of nodes in nodeset",
		},
	)
)

// IncConnections increments the total connections counter
func IncNodes() {
	nodeGauge.Inc()
}

func DecNodes() {
	nodeGauge.Dec()
}
