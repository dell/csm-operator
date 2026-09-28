// Copyright © 2021-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package constants

import (
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Constants for driver states etc
const (
	RetryCount           = 3
	Running              = csmv1.CSMStateType("Running")
	Succeeded            = csmv1.CSMStateType("Succeeded")
	Creating             = csmv1.CSMStateType("Creating")
	Failed               = csmv1.CSMStateType("Failed")
	InvalidConfig        = csmv1.CSMStateType("InvalidConfig")
	NoState              = csmv1.CSMStateType("")
	Updating             = csmv1.CSMStateType("Updating")
	Pending              = csmv1.CSMStateType("Pending")
	DefaultRetryInterval = 5 * time.Second
	MaxRetryInterval     = 10 * time.Minute
	MaxRetryDuration     = 30 * time.Minute
)

// Minimum Version Requirement for Features
const (
	DisasterRecoveryMinVersion = "v2.16.0"
)

// DriverReplicas - Replica count for controller
var DriverReplicas = int32(1)

// RevisionHistoryLimit - Max revision history limit for driver daemonset
var RevisionHistoryLimit = int32(10)

// MaxUnavailableUpdateStrategy - Maximum unavailable update strategy
var MaxUnavailableUpdateStrategy = intstr.IntOrString{IntVal: 1, StrVal: "1"}

// TerminationMessagePath for the container
const TerminationMessagePath = "/dev/termination-log"

// TerminationMessagePolicy determines the policy for termination message
const TerminationMessagePolicy = "File"

// DriverMountPath - Mount path for the driver container
const DriverMountPath = "/var/run/csi"

// DriverMountName - Socket directory volume mount name
const DriverMountName = "socket-dir"

// TerminationGracePeriodSeconds - grace period in seconds
var TerminationGracePeriodSeconds = int64(30)

// Reason - pod status
var Reason = "Reason"

// ContainerCreating - pod container
var ContainerCreating = "ContainerCreating"

// PendingCreate - pod pending
var PendingCreate = "Pending create"

// PodStatusRemoveString -make pod status crisp
var PodStatusRemoveString = "rpc error: code = Unknown desc = Error response from daemon:"

// CsmLabel - label driver resources
var CsmLabel = "csm"

// CsmNamespaceLabel - label to track namespace for csm
var CsmNamespaceLabel = "csmNamespace"

// NotFoundMsg - error message
var NotFoundMsg = "not found"

// Metrics Constants
const (
	// CsiMetricsEnabled - master switch for the shared Prometheus metrics endpoint
	CsiMetricsEnabled = "<X_CSI_METRICS_ENABLED>"

	// CsiMetricsPort - port for the shared Prometheus metrics endpoint
	CsiMetricsPort = "<X_CSI_METRICS_PORT>"

	// CsiMetricsLeaderElectionEnabled - leader election for metrics
	CsiMetricsLeaderElectionEnabled = "<X_CSI_METRICS_LEADER_ELECTION_ENABLED>"

	// CsiMetricsLeaderElectionLeaseDuration - leader election lease duration
	CsiMetricsLeaderElectionLeaseDuration = "<X_CSI_METRICS_LEADER_ELECTION_LEASE_DURATION>"

	// CsiMetricsLeaderElectionRenewDeadline - leader election renew deadline
	CsiMetricsLeaderElectionRenewDeadline = "<X_CSI_METRICS_LEADER_ELECTION_RENEW_DEADLINE>"

	// CsiMetricsLeaderElectionRetryPeriod - leader election retry period
	CsiMetricsLeaderElectionRetryPeriod = "<X_CSI_METRICS_LEADER_ELECTION_RETRY_PERIOD>"

	// CsiMetricsTLSCertFile - TLS cert file path
	CsiMetricsTLSCertFile = "<X_CSI_METRICS_TLS_CERT_FILE>"

	// CsiMetricsTLSKeyFile - TLS key file path
	CsiMetricsTLSKeyFile = "<X_CSI_METRICS_TLS_KEY_FILE>"

	// CsiMetricsCollectionInterval - interval for metrics collection
	CsiMetricsCollectionInterval = "<X_CSI_METRICS_COLLECTION_INTERVAL>"

	// CsiMetricsCollectionCacheTTL - cache TTL for metrics collection
	CsiMetricsCollectionCacheTTL = "<X_CSI_METRICS_COLLECTION_CACHE_TTL>"

	// CsiMetricsArrayRateLimit - rate limit for metrics array requests
	CsiMetricsArrayRateLimit = "<X_CSI_METRICS_ARRAY_RATE_LIMIT>"

	// CsiMetricsArrayTimeout - timeout for metrics array requests
	CsiMetricsArrayTimeout = "<X_CSI_METRICS_ARRAY_TIMEOUT>"

	// CsiMetricsArrayCBThreshold - metrics array circuit breaker failure threshold
	CsiMetricsArrayCBThreshold = "<X_CSI_METRICS_ARRAY_CB_THRESHOLD>"

	// CsiMetricsArrayCBResetTimeout - metrics array circuit breaker reset timeout
	CsiMetricsArrayCBResetTimeout = "<X_CSI_METRICS_ARRAY_CB_RESET_TIMEOUT>"

	// CsiIsiTLSHandshakeTimeout - TLS handshake timeout for NFS mTLS
	CsiIsiTLSHandshakeTimeout = "<X_CSI_ISI_TLS_HANDSHAKE_TIMEOUT_SECONDS>"
)
