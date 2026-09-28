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

package drivers

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/constants"
	"github.com/dell/csm-operator/pkg/logger"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// +kubebuilder:scaffold:imports
)

const (
	// PowerScalePluginIdentifier -
	PowerScalePluginIdentifier = "powerscale"

	// PowerScaleConfigParamsVolumeMount -
	PowerScaleConfigParamsVolumeMount = "csi-isilon-config-params" // #nosec G101

	// PowerScaleConfigVolumeMount -
	PowerScaleConfigVolumeMount = "isilon-configs"

	// PowerScaleDebug - will be used to control the GOISILON_DEBUG variable
	PowerScaleDebug string = "<GOISILON_DEBUG>"

	// PowerScaleCsiVolPrefix - will be used to control the CSI_VOL_PREFIX variable
	PowerScaleCsiVolPrefix string = "<CSI_VOL_PREFIX>"

	PowerScaleResizerHTTPEndpointArg     string = "<X_CSI_RESIZER_HTTP_ENDPOINT_ARG>"
	PowerScaleAttacherHTTPEndpointArg    string = "<X_CSI_ATTACHER_HTTP_ENDPOINT_ARG>"
	PowerScaleProvisionerHTTPEndpointArg string = "<X_CSI_PROVISIONER_HTTP_ENDPOINT_ARG>"
	PowerScaleSnapshotterHTTPEndpointArg string = "<X_CSI_SNAPSHOTTER_HTTP_ENDPOINT_ARG>"

	powerScaleDefaultMetricsCollectionInterval = "30s"
	powerScaleDefaultMetricsCollectionCacheTTL = "25s"
	powerScaleDefaultMetricsArrayRateLimit     = int32(100)
	powerScaleDefaultMetricsArrayTimeout       = "30s"
	powerScaleDefaultMetricsArrayCBThreshold   = int32(3)
	powerScaleDefaultMetricsArrayCBReset       = "30s"
)

// PrecheckPowerScale do input validation
func PrecheckPowerScale(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, ct client.Client) error {
	log := logger.GetLogger(ctx)
	// Check for secret only
	config := cr.Name + "-creds"

	if cr.Spec.Driver.AuthSecret != "" {
		config = cr.Spec.Driver.AuthSecret
	}

	version, err := operatorutils.GetVersion(ctx, cr, operatorConfig)
	if err != nil {
		return err
	}
	// Check if driver version is supported by doing a stat on a config file
	configFilePath := fmt.Sprintf("%s/driverconfig/%s/%s/driver-config-params.yaml", operatorConfig.ConfigDirectory, csmv1.PowerScaleName, version)
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Errorw("PreCheckPowerScale failed in version check", "Error", err.Error())
		return fmt.Errorf("%s %s not supported", csmv1.PowerScaleName, version)
	}

	// check if skip validation is enabled:
	skipCertValid := false
	certCount := 1
	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION" {
				b, err := strconv.ParseBool(env.Value)
				if err != nil {
					return fmt.Errorf("%s is an invalid value for X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION: %v", env.Value, err)
				}
				skipCertValid = b
			}
			if env.Name == "CERT_SECRET_COUNT" {
				d, err := strconv.ParseInt(env.Value, 0, 8)
				if err != nil {
					return fmt.Errorf("%s is an invalid value for CERT_SECRET_COUNT: %v", env.Value, err)
				}
				certCount = int(d)
			}
		}
	}

	secrets := []string{config}

	log.Debugw("preCheck", "skipCertValid", skipCertValid, "certCount", certCount, "secrets", len(secrets))

	if !skipCertValid {
		for i := 0; i < certCount; i++ {
			secrets = append(secrets, fmt.Sprintf("%s-certs-%d", cr.Name, i))
		}
	}

	for _, name := range secrets {
		found := &corev1.Secret{}
		err := ct.Get(ctx, types.NamespacedName{Name: name, Namespace: cr.GetNamespace()}, found)
		if err != nil {
			log.Error(err, "Failed query for secret ", name)
			if errors.IsNotFound(err) {
				return fmt.Errorf("failed to find secret %s", name)
			}
		}
	}

	if isDriverMetricsTLSEnabled(*cr) {
		found := &corev1.Secret{}
		secretName := cr.Spec.Driver.Metrics.TLSCertSecret
		err := ct.Get(ctx, types.NamespacedName{Name: secretName, Namespace: cr.GetNamespace()}, found)
		if err != nil {
			log.Error(err, "Failed query for secret", secretName, "Namespace", cr.Namespace)
			if errors.IsNotFound(err) {
				return fmt.Errorf("failed to find secret %s", secretName)
			}
		}
	}

	return nil
}

func getApplyCertVolume(cr csmv1.ContainerStorageModule) (*acorev1.VolumeApplyConfiguration, error) {
	skipCertValid := false
	certCount := 1

	if cr.Spec.Driver.Common != nil {
		if len(cr.Spec.Driver.Common.Envs) == 0 ||
			(len(cr.Spec.Driver.Common.Envs) == 1 && cr.Spec.Driver.Common.Envs[0].Name != "CERT_SECRET_COUNT") {
			certCount = 0
		}

		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION" {
				b, err := strconv.ParseBool(env.Value)
				if err != nil {
					return nil, fmt.Errorf("%s is an invalid value for X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION: %v", env.Value, err)
				}
				skipCertValid = b
			}
			if env.Name == "CERT_SECRET_COUNT" {
				d, err := strconv.ParseInt(env.Value, 0, 8)
				if err != nil {
					return nil, fmt.Errorf("%s is an invalid value for CERT_SECRET_COUNT: %v", env.Value, err)
				}
				certCount = int(d)
			}
		}
	} else {
		certCount = 0
		skipCertValid = true
	}

	name := "certs"
	volume := acorev1.VolumeApplyConfiguration{
		Name: &name,
		VolumeSourceApplyConfiguration: acorev1.VolumeSourceApplyConfiguration{
			Projected: &acorev1.ProjectedVolumeSourceApplyConfiguration{
				Sources: []acorev1.VolumeProjectionApplyConfiguration{},
			},
		},
	}

	if !skipCertValid {
		for i := 0; i < certCount; i++ {
			localname := fmt.Sprintf("%s-certs-%d", cr.Name, i)
			value := fmt.Sprintf("cert-%d", i)
			source := acorev1.SecretProjectionApplyConfiguration{
				LocalObjectReferenceApplyConfiguration: acorev1.LocalObjectReferenceApplyConfiguration{Name: &localname},
				Items: []acorev1.KeyToPathApplyConfiguration{
					{
						Key:  &value,
						Path: &value,
					},
				},
			}
			volume.VolumeSourceApplyConfiguration.Projected.Sources = append(volume.VolumeSourceApplyConfiguration.Projected.Sources, acorev1.VolumeProjectionApplyConfiguration{Secret: &source})

		}
	}

	return &volume, nil
}

// ModifyPowerScaleCR - It modifies the CR of powerscale/isilon (currently for Storage Capacity Tracking
func ModifyPowerScaleCR(yamlString string, cr csmv1.ContainerStorageModule, fileType string) string {
	// Parameters to initialise CR values
	storageCapacity := "false"
	healthMonitorNode := "false"
	healthMonitorController := "false"
	// GOISILON_DEBUG defaults to false
	debug := "false"
	// CSI_VOL_PREFIX defaults to csivol
	csiVolPrefix := "csivol"
	metricsEnabled := "false"
	metricsPort := "8443"
	metricsTLSCertFile := ""
	metricsTLSKeyFile := ""
	metricsCollectionInterval := powerScaleDefaultMetricsCollectionInterval
	metricsCollectionCacheTTL := powerScaleDefaultMetricsCollectionCacheTTL
	metricsArrayRateLimit := strconv.FormatInt(int64(powerScaleDefaultMetricsArrayRateLimit), 10)
	metricsArrayTimeout := powerScaleDefaultMetricsArrayTimeout
	metricsArrayCBThreshold := strconv.FormatInt(int64(powerScaleDefaultMetricsArrayCBThreshold), 10)
	metricsArrayCBResetTimeout := powerScaleDefaultMetricsArrayCBReset
	metricsLeaderElectionEnabled := "false"
	metricsLeaderElectionLeaseDuration := "60s"
	metricsLeaderElectionRenewDeadline := "40s"
	metricsLeaderElectionRetryPeriod := "5s"
	resizerHTTPEndpointArg := ""
	attacherHTTPEndpointArg := ""
	provisionerHTTPEndpointArg := ""
	snapshotterHTTPEndpointArg := ""
	tlsHandshakeTimeout := "30"

	if cr.Spec.Driver.Common != nil {
		for _, env := range cr.Spec.Driver.Common.Envs {
			if env.Name == "GOISILON_DEBUG" {
				debug = env.Value
			}
			if env.Name == "X_CSI_ISI_TLS_HANDSHAKE_TIMEOUT_SECONDS" {
				tlsHandshakeTimeout = env.Value
			}
		}
	}

	if cr.Spec.Driver.Metrics != nil {
		if cr.Spec.Driver.Metrics.Enabled {
			metricsEnabled = "true"
			attacherHTTPEndpointArg = "--http-endpoint=:8081"
			resizerHTTPEndpointArg = "--http-endpoint=:8082"
			snapshotterHTTPEndpointArg = "--http-endpoint=:8083"
			provisionerHTTPEndpointArg = "--http-endpoint=:8084"
		}
		if cr.Spec.Driver.Metrics.Port != 0 {
			metricsPort = fmt.Sprintf("%d", cr.Spec.Driver.Metrics.Port)
		}
		if isDriverMetricsTLSEnabled(cr) {
			metricsTLSCertFile = "/etc/metrics-tls/tls.crt"
			metricsTLSKeyFile = "/etc/metrics-tls/tls.key"
		}
		if cr.Spec.Driver.Metrics.Collection != nil {
			metricsCollectionInterval = metricsDurationOrDefault(cr.Spec.Driver.Metrics.Collection.Interval, powerScaleDefaultMetricsCollectionInterval)
			metricsCollectionCacheTTL = metricsDurationOrDefault(cr.Spec.Driver.Metrics.Collection.CacheTTL, powerScaleDefaultMetricsCollectionCacheTTL)
		}
		if cr.Spec.Driver.Metrics.LeaderElection != nil {
			if cr.Spec.Driver.Metrics.LeaderElection.Enabled != nil {
				if *cr.Spec.Driver.Metrics.LeaderElection.Enabled {
					metricsLeaderElectionEnabled = "true"
				}
			}
			metricsLeaderElectionLeaseDuration, metricsLeaderElectionRenewDeadline, metricsLeaderElectionRetryPeriod = metricsLeaderElectionDurationDefaultsOrSanitized(
				cr.Spec.Driver.Metrics.LeaderElection.LeaseDuration,
				cr.Spec.Driver.Metrics.LeaderElection.RenewDeadline,
				cr.Spec.Driver.Metrics.LeaderElection.RetryPeriod,
				metricsLeaderElectionLeaseDuration,
				metricsLeaderElectionRenewDeadline,
				metricsLeaderElectionRetryPeriod,
			)
		}
		if cr.Spec.Driver.Metrics.Array != nil {
			metricsArrayRateLimit = metricsPositiveIntOrDefault(cr.Spec.Driver.Metrics.Array.RateLimit, powerScaleDefaultMetricsArrayRateLimit)
			metricsArrayTimeout = metricsDurationOrDefault(cr.Spec.Driver.Metrics.Array.Timeout, powerScaleDefaultMetricsArrayTimeout)
			if cr.Spec.Driver.Metrics.Array.CircuitBreaker != nil {
				metricsArrayCBThreshold = metricsPositiveIntOrDefault(cr.Spec.Driver.Metrics.Array.CircuitBreaker.Threshold, powerScaleDefaultMetricsArrayCBThreshold)
				metricsArrayCBResetTimeout = metricsDurationOrDefault(cr.Spec.Driver.Metrics.Array.CircuitBreaker.ResetTimeout, powerScaleDefaultMetricsArrayCBReset)
			}
		}
	}

	switch fileType {
	case "CSIDriverSpec":
		if cr.Spec.Driver.CSIDriverSpec != nil && cr.Spec.Driver.CSIDriverSpec.StorageCapacity {
			storageCapacity = "true"
		}
		yamlString = strings.ReplaceAll(yamlString, CsiStorageCapacityEnabled, storageCapacity)
	case "Controller":
		if cr.Spec.Driver.Controller != nil {
			for _, env := range cr.Spec.Driver.Controller.Envs {
				if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
					healthMonitorController = env.Value
				}
				if env.Name == "X_CSI_VOL_PREFIX" {
					csiVolPrefix = env.Value
				}
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorController)
		yamlString = strings.ReplaceAll(yamlString, CSMNameSpace, cr.Namespace)
		yamlString = strings.ReplaceAll(yamlString, PowerScaleDebug, debug)
		yamlString = strings.ReplaceAll(yamlString, PowerScaleCsiVolPrefix, csiVolPrefix) // applicable only for v2.14.0/controller.yaml
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsEnabled, metricsEnabled)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsPort, metricsPort)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsTLSCertFile, metricsTLSCertFile)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsTLSKeyFile, metricsTLSKeyFile)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsLeaderElectionEnabled, metricsLeaderElectionEnabled)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsLeaderElectionLeaseDuration, metricsLeaderElectionLeaseDuration)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsLeaderElectionRenewDeadline, metricsLeaderElectionRenewDeadline)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsLeaderElectionRetryPeriod, metricsLeaderElectionRetryPeriod)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsCollectionInterval, metricsCollectionInterval)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsCollectionCacheTTL, metricsCollectionCacheTTL)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayRateLimit, metricsArrayRateLimit)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayTimeout, metricsArrayTimeout)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayCBThreshold, metricsArrayCBThreshold)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayCBResetTimeout, metricsArrayCBResetTimeout)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiIsiTLSHandshakeTimeout, tlsHandshakeTimeout)
		yamlString = SubstituteOptionalYAMLLine(yamlString, PowerScaleResizerHTTPEndpointArg, resizerHTTPEndpointArg)
		yamlString = SubstituteOptionalYAMLLine(yamlString, PowerScaleAttacherHTTPEndpointArg, attacherHTTPEndpointArg)
		yamlString = SubstituteOptionalYAMLLine(yamlString, PowerScaleProvisionerHTTPEndpointArg, provisionerHTTPEndpointArg)
		yamlString = SubstituteOptionalYAMLLine(yamlString, PowerScaleSnapshotterHTTPEndpointArg, snapshotterHTTPEndpointArg)
	case "Node":
		if cr.Spec.Driver.Node != nil {
			for _, env := range cr.Spec.Driver.Node.Envs {
				if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
					healthMonitorNode = env.Value
				}
			}
		}
		yamlString = strings.ReplaceAll(yamlString, CsiHealthMonitorEnabled, healthMonitorNode)
		yamlString = strings.ReplaceAll(yamlString, CSMNameSpace, cr.Namespace)
		yamlString = strings.ReplaceAll(yamlString, PowerScaleDebug, debug)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsEnabled, metricsEnabled)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsPort, metricsPort)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsTLSCertFile, metricsTLSCertFile)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsTLSKeyFile, metricsTLSKeyFile)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsLeaderElectionEnabled, metricsLeaderElectionEnabled)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsLeaderElectionLeaseDuration, metricsLeaderElectionLeaseDuration)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsLeaderElectionRenewDeadline, metricsLeaderElectionRenewDeadline)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsLeaderElectionRetryPeriod, metricsLeaderElectionRetryPeriod)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsCollectionInterval, metricsCollectionInterval)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsCollectionCacheTTL, metricsCollectionCacheTTL)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayRateLimit, metricsArrayRateLimit)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayTimeout, metricsArrayTimeout)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayCBThreshold, metricsArrayCBThreshold)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayCBResetTimeout, metricsArrayCBResetTimeout)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiIsiTLSHandshakeTimeout, tlsHandshakeTimeout)
	}
	return yamlString
}

func metricsDurationOrDefault(v, defaultValue string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return defaultValue
	}
	return v
}

func metricsPositiveIntOrDefault(v, defaultValue int32) string {
	if v <= 0 {
		return strconv.FormatInt(int64(defaultValue), 10)
	}
	return strconv.FormatInt(int64(v), 10)
}

func metricsLeaderElectionDurationDefaultsOrSanitized(leaseDuration, renewDeadline, retryPeriod, defaultLease, defaultRenew, defaultRetry string) (string, string, string) {
	sanitizedLease := metricsDurationOrDefault(leaseDuration, defaultLease)
	sanitizedRenew := metricsDurationOrDefault(renewDeadline, defaultRenew)
	sanitizedRetry := metricsDurationOrDefault(retryPeriod, defaultRetry)

	lease, leaseErr := time.ParseDuration(sanitizedLease)
	renew, renewErr := time.ParseDuration(sanitizedRenew)
	retry, retryErr := time.ParseDuration(sanitizedRetry)
	if leaseErr != nil || renewErr != nil || retryErr != nil {
		return defaultLease, defaultRenew, defaultRetry
	}

	if lease <= renew || renew <= retry {
		return defaultLease, defaultRenew, defaultRetry
	}

	return sanitizedLease, sanitizedRenew, sanitizedRetry
}
