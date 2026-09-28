// Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// OperatorMetrics holds Prometheus metrics for the csm-operator.
// All metrics include cluster_id label for multi-cluster aggregation via Thanos/Cortex.
type OperatorMetrics struct {
	// ClustersManagedTotal tracks the count of K8s clusters under management.
	// In single-operator-per-cluster model, this is always 1.
	// Aggregate across clusters using: count(dell_csm_operator_clusters_managed_total)
	ClustersManagedTotal *prometheus.GaugeVec

	// ClusterConnectivity tracks connectivity to the K8s API server (always 1 if operator is running).
	ClusterConnectivity *prometheus.GaugeVec

	// ReconciliationTotal counts reconciliation attempts by driver, module, and status.
	ReconciliationTotal *prometheus.CounterVec

	// ReconciliationDuration measures reconciliation latency.
	ReconciliationDuration *prometheus.HistogramVec

	// CRCount tracks CR count by driver and state.
	CRCount *prometheus.GaugeVec
}

// NewOperatorMetrics creates and registers csm-operator metrics with reg.
func NewOperatorMetrics(reg prometheus.Registerer) *OperatorMetrics {
	m := &OperatorMetrics{
		ClustersManagedTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_csm_operator_clusters_managed_total",
			Help: "Count of Kubernetes clusters under management (1 per operator instance).",
		}, []string{"cluster_id"}),
		ClusterConnectivity: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_csm_operator_cluster_connectivity",
			Help: "Connectivity to K8s API server (1=connected, 0=disconnected).",
		}, []string{"cluster_id"}),
		ReconciliationTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dell_csm_operator_reconciliation_total",
			Help: "Total reconciliation attempts.",
		}, []string{"cluster_id", "driver", "module", "status"}),
		ReconciliationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "dell_csm_operator_reconciliation_duration_seconds",
			Help:    "Reconciliation duration.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10},
		}, []string{"cluster_id", "driver", "module"}),
		CRCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_csm_operator_cr_count",
			Help: "CR count by driver and state.",
		}, []string{"cluster_id", "driver", "state"}),
	}
	collectors := []prometheus.Collector{
		m.ClustersManagedTotal, m.ClusterConnectivity,
		m.ReconciliationTotal, m.ReconciliationDuration, m.CRCount,
	}
	for _, c := range collectors {
		if err := reg.Register(c); err != nil {
			var are prometheus.AlreadyRegisteredError
			if errors.As(err, &are) {
				continue
			}
			panic(err)
		}
	}
	return m
}

// RecordReconciliation records a reconciliation event with cluster context.
func (m *OperatorMetrics) RecordReconciliation(clusterID, driver, module, status string, d time.Duration) {
	if m == nil {
		return
	}
	m.ReconciliationTotal.WithLabelValues(clusterID, driver, module, status).Inc()
	m.ReconciliationDuration.WithLabelValues(clusterID, driver, module).Observe(d.Seconds())
}

// SetCRCount sets the CR count gauge for a specific cluster.
func (m *OperatorMetrics) SetCRCount(clusterID, driver, state string, n float64) {
	if m == nil {
		return
	}
	m.CRCount.WithLabelValues(clusterID, driver, state).Set(n)
}

// SetClusterConnectivity sets K8s API server connectivity status.
// This is always 1 if the operator is running (since it requires API access).
func (m *OperatorMetrics) SetClusterConnectivity(clusterID string, connected bool) {
	if m == nil {
		return
	}
	v := 0.0
	if connected {
		v = 1.0
	}
	m.ClusterConnectivity.WithLabelValues(clusterID).Set(v)
}

// SetClustersManaged sets the clusters managed count for this operator instance.
// In single-operator-per-cluster model, this should be 1.
func (m *OperatorMetrics) SetClustersManaged(clusterID string, n float64) {
	if m == nil {
		return
	}
	m.ClustersManagedTotal.WithLabelValues(clusterID).Set(n)
}
