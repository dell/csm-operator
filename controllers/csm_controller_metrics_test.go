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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gatherOprMetric(t *testing.T, reg prometheus.Gatherer, name string) *dto.MetricFamily {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

func counterOpr(mf *dto.MetricFamily, labels map[string]string) float64 {
	if mf == nil {
		return 0
	}
	for _, m := range mf.GetMetric() {
		got := make(map[string]string)
		for _, lp := range m.GetLabel() {
			got[lp.GetName()] = lp.GetValue()
		}
		match := true
		for k, v := range labels {
			if got[k] != v {
				match = false
				break
			}
		}
		if match {
			return m.GetCounter().GetValue()
		}
	}
	return 0
}

func gaugeOpr(mf *dto.MetricFamily, labels map[string]string) (float64, bool) {
	if mf == nil {
		return 0, false
	}
	for _, m := range mf.GetMetric() {
		got := make(map[string]string)
		for _, lp := range m.GetLabel() {
			got[lp.GetName()] = lp.GetValue()
		}
		match := true
		for k, v := range labels {
			if got[k] != v {
				match = false
				break
			}
		}
		if match {
			return m.GetGauge().GetValue(), true
		}
	}
	return 0, false
}

// U-OPR-01: RecordReconciliation increments reconciliation_total with cluster_id
func TestOperatorMetrics_RecordReconciliation(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewOperatorMetrics(reg)

	m.RecordReconciliation("test-cluster", "powerflex", "metrics", "success", 300*time.Millisecond)

	mf := gatherOprMetric(t, reg, "dell_csm_operator_reconciliation_total")
	require.NotNil(t, mf, "reconciliation_total must be registered")
	v := counterOpr(mf, map[string]string{"cluster_id": "test-cluster", "driver": "powerflex", "module": "metrics", "status": "success"})
	assert.Equal(t, 1.0, v)

	durMF := gatherOprMetric(t, reg, "dell_csm_operator_reconciliation_duration_seconds")
	require.NotNil(t, durMF)
}

// U-OPR-02: SetCRCount sets the cr_count gauge with cluster_id
func TestOperatorMetrics_SetCRCount(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewOperatorMetrics(reg)

	m.SetCRCount("test-cluster", "powerflex", "Running", 3)

	mf := gatherOprMetric(t, reg, "dell_csm_operator_cr_count")
	require.NotNil(t, mf)
	v, ok := gaugeOpr(mf, map[string]string{"cluster_id": "test-cluster", "driver": "powerflex", "state": "Running"})
	require.True(t, ok)
	assert.Equal(t, 3.0, v)
}

// U-OPR-03: SetClusterConnectivity(false) → gauge = 0 with cluster_id
func TestOperatorMetrics_SetClusterConnectivity_Disconnected(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewOperatorMetrics(reg)

	m.SetClusterConnectivity("test-cluster", false)

	mf := gatherOprMetric(t, reg, "dell_csm_operator_cluster_connectivity")
	require.NotNil(t, mf)
	v, ok := gaugeOpr(mf, map[string]string{"cluster_id": "test-cluster"})
	require.True(t, ok)
	assert.Equal(t, 0.0, v)
}

// U-OPR-04: SetClustersManaged sets the clusters managed count with cluster_id
func TestOperatorMetrics_SetClustersManaged(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewOperatorMetrics(reg)

	m.SetClustersManaged("test-cluster", 1)

	mf := gatherOprMetric(t, reg, "dell_csm_operator_clusters_managed_total")
	require.NotNil(t, mf)
	v, ok := gaugeOpr(mf, map[string]string{"cluster_id": "test-cluster"})
	require.True(t, ok)
	assert.Equal(t, 1.0, v)
}

// U-OPR-05: NewOperatorMetrics handles AlreadyRegisteredError gracefully
func TestNewOperatorMetrics_AlreadyRegisteredError_Ignores(t *testing.T) {
	reg := prometheus.NewRegistry()

	// First registration should succeed
	m1 := NewOperatorMetrics(reg)
	require.NotNil(t, m1)

	// Second registration should handle AlreadyRegisteredError gracefully
	m2 := NewOperatorMetrics(reg)
	require.NotNil(t, m2)
}

// U-OPR-06: NewOperatorMetrics panics on non-recoverable registration errors
func TestNewOperatorMetrics_PanicOnRegistrationError(t *testing.T) {
	// Create a custom registerer that always returns a non-recoverable error
	errorReg := &errorRegisterer{
		registerErr: errors.New("simulated registration error"),
	}

	assert.Panics(t, func() {
		NewOperatorMetrics(errorReg)
	})
}

// U-OPR-07: RecordReconciliation with nil metrics is safe
func TestOperatorMetrics_RecordReconciliation_NilMetrics_Safe(_ *testing.T) {
	var m *OperatorMetrics

	// Should not panic when called with nil receiver
	m.RecordReconciliation("test-cluster", "powerflex", "metrics", "success", 300*time.Millisecond)
}

// U-OPR-08: SetCRCount with nil metrics is safe
func TestOperatorMetrics_SetCRCount_NilMetrics_Safe(_ *testing.T) {
	var m *OperatorMetrics

	// Should not panic when called with nil receiver
	m.SetCRCount("test-cluster", "powerflex", "Running", 3)
}

// U-OPR-09: SetClusterConnectivity with nil metrics is safe
func TestOperatorMetrics_SetClusterConnectivity_NilMetrics_Safe(_ *testing.T) {
	var m *OperatorMetrics

	// Should not panic when called with nil receiver
	m.SetClusterConnectivity("test-cluster", true)
}

// U-OPR-10: SetClustersManaged with nil metrics is safe
func TestOperatorMetrics_SetClustersManaged_NilMetrics_Safe(_ *testing.T) {
	var m *OperatorMetrics

	// Should not panic when called with nil receiver
	m.SetClustersManaged("test-cluster", 1)
}

// U-OPR-11: RecordReconciliation increments counter multiple times
func TestOperatorMetrics_RecordReconciliation_MultipleIncrements(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewOperatorMetrics(reg)

	m.RecordReconciliation("test-cluster", "powerflex", "metrics", "success", 300*time.Millisecond)
	m.RecordReconciliation("test-cluster", "powerflex", "metrics", "success", 400*time.Millisecond)
	m.RecordReconciliation("test-cluster", "powerflex", "metrics", "success", 500*time.Millisecond)

	mf := gatherOprMetric(t, reg, "dell_csm_operator_reconciliation_total")
	require.NotNil(t, mf)
	v := counterOpr(mf, map[string]string{"cluster_id": "test-cluster", "driver": "powerflex", "module": "metrics", "status": "success"})
	assert.Equal(t, 3.0, v)
}

// U-OPR-12: RecordReconciliation with different labels creates separate counters
func TestOperatorMetrics_RecordReconciliation_DifferentLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewOperatorMetrics(reg)

	m.RecordReconciliation("test-cluster", "powerflex", "metrics", "success", 300*time.Millisecond)
	m.RecordReconciliation("test-cluster", "powermax", "metrics", "success", 400*time.Millisecond)
	m.RecordReconciliation("test-cluster", "powerflex", "metrics", "failure", 500*time.Millisecond)

	mf := gatherOprMetric(t, reg, "dell_csm_operator_reconciliation_total")
	require.NotNil(t, mf)

	v1 := counterOpr(mf, map[string]string{"cluster_id": "test-cluster", "driver": "powerflex", "module": "metrics", "status": "success"})
	assert.Equal(t, 1.0, v1)

	v2 := counterOpr(mf, map[string]string{"cluster_id": "test-cluster", "driver": "powermax", "module": "metrics", "status": "success"})
	assert.Equal(t, 1.0, v2)

	v3 := counterOpr(mf, map[string]string{"cluster_id": "test-cluster", "driver": "powerflex", "module": "metrics", "status": "failure"})
	assert.Equal(t, 1.0, v3)
}

// U-OPR-13: SetClusterConnectivity sets correct gauge values
func TestOperatorMetrics_SetClusterConnectivity_CorrectValues(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewOperatorMetrics(reg)

	m.SetClusterConnectivity("test-cluster", true)
	mf := gatherOprMetric(t, reg, "dell_csm_operator_cluster_connectivity")
	require.NotNil(t, mf)
	v, ok := gaugeOpr(mf, map[string]string{"cluster_id": "test-cluster"})
	require.True(t, ok)
	assert.Equal(t, 1.0, v)

	m.SetClusterConnectivity("test-cluster", false)
	mf = gatherOprMetric(t, reg, "dell_csm_operator_cluster_connectivity")
	require.NotNil(t, mf)
	v, ok = gaugeOpr(mf, map[string]string{"cluster_id": "test-cluster"})
	require.True(t, ok)
	assert.Equal(t, 0.0, v)
}

// errorRegisterer is a custom registerer that always returns an error
type errorRegisterer struct {
	registerErr error
}

func (e *errorRegisterer) Register(_ prometheus.Collector) error {
	return e.registerErr
}

func (e *errorRegisterer) MustRegister(_ ...prometheus.Collector) {
	panic(e.registerErr)
}

func (e *errorRegisterer) Unregister(_ prometheus.Collector) bool {
	return false
}
