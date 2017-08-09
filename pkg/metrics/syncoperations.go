package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	phasePending      = "pending"
	phaseProvisioning = "provisioning"
	phaseLaunching    = "launching"
	phaseRunning      = "running"
	phaseDeleting     = "deleting"
)

var (
	syncPendingNodeDurationsHisto = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: Prefix + "sync_pending_node_durations_histogram_seconds",
		Help: "Sync pedning node duration.",
	})
	syncProvisioningNodeDurationsHisto = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: Prefix + "sync_provision_node_durations_histogram_seconds",
		Help: "Sync provisioning node duration.",
	})
	syncLaunchingNodeDurationsHisto = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: Prefix + "sync_launching_node_durations_histogram_seconds",
		Help: "Sync launching node duration.",
	})
	syncDeletingNodeDurationsHisto = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: Prefix + "sync_deleteting_node_durations_histogram_seconds",
		Help: "Sync deleting node duration.",
	})
)

// ConnectionTime gather the duration of a connection
func SyncOperationTime(d time.Duration, phase string) {
	switch phase {
	case phasePending:
		syncPendingNodeDurationsHisto.Observe(d.Seconds())
	case phaseProvisioning:
		syncProvisioningNodeDurationsHisto.Observe(d.Seconds())
	case phaseLaunching:
		syncLaunchingNodeDurationsHisto.Observe(d.Seconds())
	case phaseDeleting:
		syncDeletingNodeDurationsHisto.Observe(d.Seconds())
	}
}
