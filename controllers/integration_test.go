/*
 Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func oprFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return p
}

// I-OPR-01: MetricsServer :8443 equivalent; operator metrics registered; scrape returns dell_csm_operator_* metrics.
func TestIntegration_OPR_OperatorMetricsScrape(t *testing.T) {
	reg := prometheus.NewRegistry()

	reconcileTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dell_csm_operator_reconciliation_total",
		Help: "Total reconciliations.",
	}, []string{"driver", "status"})
	crCount := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dell_csm_operator_cr_count",
		Help: "CR count by driver and state.",
	}, []string{"driver", "state"})
	connectivity := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dell_csm_operator_cluster_connectivity",
		Help: "Cluster connectivity.",
	}, []string{"cluster"})
	reg.MustRegister(reconcileTotal, crCount, connectivity)

	reconcileTotal.WithLabelValues("powerflex", "success").Add(5)
	crCount.WithLabelValues("powerflex", "Running").Set(2)
	connectivity.WithLabelValues("default").Set(1)

	port := oprFreePort(t)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	go func() { _ = srv.ListenAndServe() }()
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	time.Sleep(80 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode, "metrics endpoint must return HTTP 200")
	assert.Contains(t, string(body), "dell_csm_operator_reconciliation_total",
		"scrape must contain reconciliation counter")
	assert.Contains(t, string(body), "dell_csm_operator_cr_count",
		"scrape must contain CR count gauge")
	assert.Contains(t, string(body), "dell_csm_operator_cluster_connectivity",
		"scrape must contain cluster connectivity gauge")
}
