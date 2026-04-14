package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"omni-cd/internal/state"
)

// Collector implements prometheus.Collector and reads AppState on each scrape.
type Collector struct {
	appState        *state.AppState
	reconcileTotal  *prometheus.CounterVec
	clustersTotal   *prometheus.Desc
	machinesHealthy *prometheus.Desc
	machinesTotal   *prometheus.Desc
	mcTotal         *prometheus.Desc
	reconcileDur    *prometheus.Desc
	omniConnected   *prometheus.Desc
	reposTotal      *prometheus.Desc
	info            *prometheus.Desc
}

// New creates a new Collector backed by the given AppState.
func New(appState *state.AppState) *Collector {
	c := &Collector{
		appState: appState,
		reconcileTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "omni_cd_reconcile_total",
			Help: "Total number of reconcile cycles by result.",
		}, []string{"result"}),
		clustersTotal: prometheus.NewDesc(
			"omni_cd_clusters_total",
			"Number of clusters grouped by status.",
			[]string{"status"}, nil,
		),
		machinesHealthy: prometheus.NewDesc(
			"omni_cd_cluster_machines_healthy",
			"Number of healthy machines in a cluster.",
			[]string{"cluster"}, nil,
		),
		machinesTotal: prometheus.NewDesc(
			"omni_cd_cluster_machines_total",
			"Total number of machines in a cluster.",
			[]string{"cluster"}, nil,
		),
		mcTotal: prometheus.NewDesc(
			"omni_cd_machine_classes_total",
			"Number of machine classes grouped by status.",
			[]string{"status"}, nil,
		),
		reconcileDur: prometheus.NewDesc(
			"omni_cd_reconcile_last_duration_seconds",
			"Duration of the last reconcile cycle in seconds.",
			nil, nil,
		),
		omniConnected: prometheus.NewDesc(
			"omni_cd_omni_connected",
			"1 if the Omni endpoint is reachable, 0 otherwise.",
			nil, nil,
		),
		reposTotal: prometheus.NewDesc(
			"omni_cd_repos_total",
			"Number of git repositories by sync status.",
			[]string{"status"}, nil,
		),
		info: prometheus.NewDesc(
			"omni_cd_info",
			"Informational metric with app version and Omni instance details.",
			[]string{"version", "omni_endpoint", "omni_version"}, nil,
		),
	}

	// Pre-initialise counters so they appear even before the first reconcile.
	c.reconcileTotal.WithLabelValues("success")
	c.reconcileTotal.WithLabelValues("failed")

	return c
}

// RecordReconcile increments the reconcile counter for the given outcome.
func (c *Collector) RecordReconcile(success bool) {
	if success {
		c.reconcileTotal.WithLabelValues("success").Inc()
	} else {
		c.reconcileTotal.WithLabelValues("failed").Inc()
	}
}

// Describe sends all metric descriptors to the channel.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	c.reconcileTotal.Describe(ch)
	ch <- c.clustersTotal
	ch <- c.machinesHealthy
	ch <- c.machinesTotal
	ch <- c.mcTotal
	ch <- c.reconcileDur
	ch <- c.omniConnected
	ch <- c.reposTotal
	ch <- c.info
}

// Collect reads the current AppState snapshot and emits all metrics.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.reconcileTotal.Collect(ch)

	s := c.appState.Snapshot()

	// omni_cd_clusters_total — grouped by status, always emit all known statuses
	clusterStatuses := []string{"success", "outofsync", "failed", "syncing", "unmanaged", "deleting", "orphaned"}
	statusCounts := map[string]int{}
	for _, st := range clusterStatuses {
		statusCounts[st] = 0
	}
	for _, cl := range s.Clusters {
		statusCounts[cl.Status]++
		ch <- prometheus.MustNewConstMetric(
			c.machinesHealthy, prometheus.GaugeValue,
			float64(cl.MachinesHealthy), cl.ID,
		)
		ch <- prometheus.MustNewConstMetric(
			c.machinesTotal, prometheus.GaugeValue,
			float64(cl.MachinesTotal), cl.ID,
		)
	}
	for status, count := range statusCounts {
		ch <- prometheus.MustNewConstMetric(
			c.clustersTotal, prometheus.GaugeValue,
			float64(count), status,
		)
	}

	// omni_cd_machine_classes_total — grouped by status, always emit all known statuses
	mcStatuses := []string{"success", "outofsync", "failed", "syncing", "unmanaged"}
	mcCounts := map[string]int{}
	for _, st := range mcStatuses {
		mcCounts[st] = 0
	}
	for _, mc := range s.MachineClasses {
		mcCounts[mc.Status]++
	}
	for status, count := range mcCounts {
		ch <- prometheus.MustNewConstMetric(
			c.mcTotal, prometheus.GaugeValue,
			float64(count), status,
		)
	}

	// omni_cd_reconcile_last_duration_seconds
	if !s.LastReconcile.FinishedAt.IsZero() && !s.LastReconcile.StartedAt.IsZero() {
		dur := s.LastReconcile.FinishedAt.Sub(s.LastReconcile.StartedAt).Seconds()
		ch <- prometheus.MustNewConstMetric(
			c.reconcileDur, prometheus.GaugeValue, dur,
		)
	}

	// omni_cd_omni_connected
	omniVal := 0.0
	if s.OmniHealth.Status == "healthy" || s.OmniHealth.Status == "ok" {
		omniVal = 1.0
	}
	ch <- prometheus.MustNewConstMetric(c.omniConnected, prometheus.GaugeValue, omniVal)

	// omni_cd_repos_total
	repoOk, repoErr := 0, 0
	for _, r := range s.Repos {
		if r.SyncError != "" {
			repoErr++
		} else {
			repoOk++
		}
	}
	ch <- prometheus.MustNewConstMetric(
		c.reposTotal, prometheus.GaugeValue, float64(repoOk), "ok",
	)
	ch <- prometheus.MustNewConstMetric(
		c.reposTotal, prometheus.GaugeValue, float64(repoErr), "error",
	)

	// omni_cd_info
	ch <- prometheus.MustNewConstMetric(
		c.info, prometheus.GaugeValue, 1, s.AppVersion, s.OmniEndpoint, s.OmniVersion,
	)
}
