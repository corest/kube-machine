package metrics

import "github.com/prometheus/client_golang/prometheus"

const (
	Prefix = "kubenode_"
)

func init() {
	prometheus.MustRegister(nodeGauge)
	prometheus.MustRegister(errorCounterVec)
	prometheus.MustRegister(syncPendingNodeDurationsHisto)
	prometheus.MustRegister(syncProvisioningNodeDurationsHisto)
	prometheus.MustRegister(syncLaunchingNodeDurationsHisto)
	prometheus.MustRegister(syncDeletingNodeDurationsHisto)
}
