//  Copyright © 2023-2025 Dell Inc. or its subsidiaries. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//       http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package drivers

import (
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/dell/csm-operator/pkg/constants"
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	v1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

// Constant to be used for powermax deployment
const (
	// PowerMaxPluginIdentifier used to identify powermax plugin
	PowerMaxPluginIdentifier     = "powermax"
	ReverseProxyServerComponent  = "csipowermax-reverseproxy" // #nosec G101
	RevProxyTLSSecretDefaultName = "csirevproxy-tls-secret"   // #nosec G101

	// PowerMaxConfigParamsVolumeMount used to identify config param volume mount
	PowerMaxConfigParamsVolumeMount = "powermax-config-params"

	// PowerMaxConfigVolumeMount -
	PowerMaxConfigVolumeMount = CSIPowerMaxSecretVolumeName

	// CSIPmaxManagedArray and following used for replacing user values in config files
	CSIPmaxManagedArray               = "<X_CSI_MANAGED_ARRAY>"
	CSIPmaxEndpoint                   = "<X_CSI_POWERMAX_ENDPOINT>"
	CSIPmaxDebug                      = "<X_CSI_POWERMAX_DEBUG>"
	CSIPmaxPortGroup                  = "<X_CSI_POWERMAX_PORTGROUPS>"
	CSIPmaxProtocol                   = "<X_CSI_TRANSPORT_PROTOCOL>"
	CSIPmaxNodeTemplate               = "<X_CSI_IG_NODENAME_TEMPLATE>"
	CSIPmaxModifyHostname             = "<X_CSI_IG_MODIFY_HOSTNAME>"
	CSIPmaxHealthMonitor              = "<X_CSI_HEALTH_MONITOR_ENABLED>"
	CSIPmaxTopology                   = "<X_CSI_TOPOLOGY_CONTROL_ENABLED>"
	CSIPmaxVsphere                    = "<X_CSI_VSPHERE_ENABLED>"
	CSIPmaxVspherePG                  = "<X_CSI_VSPHERE_PORTGROUP>"
	CSIPmaxVsphereHostname            = "<X_CSI_VSPHERE_HOSTNAME>"
	CSIPmaxVsphereHost                = "<X_CSI_VCENTER_HOST>"
	CSIPmaxChap                       = "<X_CSI_POWERMAX_ISCSI_ENABLE_CHAP>"
	ReverseProxyTLSSecret             = "<X_CSI_REVPROXY_TLS_SECRET>" // #nosec G101
	CSIPmaxDynamicSGEnabled           = "<X_CSI_DYNAMIC_SG_ENABLED>"
	CSIPmaxCSIAddonsReplEnabled       = "<X_CSI_CSIADDONS_REPLICATION_ENABLED>"
	CSIPmaxMetroSiteFailureEnabled    = "<X_CSI_POWERMAX_METRO_SITE_FAILURE_HANDLING_ENABLED>"
	CSIPmaxMetroStateCheckTimeout     = "<X_CSI_POWERMAX_METRO_STATE_CHECK_TIMEOUT>"
	CSIPmaxMetroQueueWarningThreshold = "<X_CSI_POWERMAX_METRO_QUEUE_WARNING_THRESHOLD>"
	CSIPmaxMetroQueueHardLimit        = "<X_CSI_POWERMAX_METRO_QUEUE_HARD_LIMIT>"
	CSIPmaxMetroReconciliationBackoff = "<X_CSI_POWERMAX_METRO_RECONCILIATION_BACKOFF>"
	CSIPmaxDriverInstanceUID          = "<X_CSI_DRIVER_INSTANCE_UID>"
	CSIPmaxCapacityPollInterval       = "<X_CSI_CAPACITY_POLL_INTERVAL>"
	CSIPmaxCapacityThresholdFull      = "<X_CSI_CAPACITY_THRESHOLD_FULL>"

	// CsiPmaxMaxVolumesPerNode - Maximum volumes that the controller can schedule on the node
	CsiPmaxMaxVolumesPerNode = "<X_CSI_MAX_VOLUMES_PER_NODE>"

	CSIPowerMaxUseSecret       string = "X_CSI_REVPROXY_USE_SECRET"      // #nosec G101
	CSIPowerMaxSecretFilePath  string = "X_CSI_REVPROXY_SECRET_FILEPATH" // #nosec G101
	CSIPowerMaxSecretMountPath string = "/etc/powermax"                  // #nosec G101

	CSIPowerMaxSecretName       string = "config"                       // #nosec G101
	CSIPowerMaxSecretVolumeName string = "powermax-reverseproxy-secret" // #nosec G101

	CSIPowerMaxConfigPathKey   string = "X_CSI_POWERMAX_CONFIG_PATH"
	CSIPowerMaxConfigPathValue string = "/powermax-config-params/driver-config-params.yaml"

	PowerMaxMountCredentialMinVersion string = "v2.14.0"

	CSIPowerMaxProxyAuthTokenFile       string = "X_CSI_REVPROXY_AUTH_TOKEN_FILE"   // #nosec G101
	CSIPowerMaxProxyAuthTokenSecretName string = "X_CSI_REVPROXY_AUTH_TOKEN_SECRET" // #nosec G101
	CSIPowerMaxProxyAuthTokenMountPath  string = "/etc/proxy-auth-token"            // #nosec G101
	CSIPowerMaxProxyAuthTokenVolumeName string = "proxy-auth-token"                 // #nosec G101
	CSIPowerMaxProxyAuthTokenKey        string = "token"                            // #nosec G101

	// PowerMax metrics default values
	powerMaxDefaultMetricsCollectionInterval  = "30s"
	powerMaxDefaultMetricsCollectionCacheTTL  = "25s"
	powerMaxDefaultMetricsArrayRateLimit      = int32(100)
	powerMaxDefaultMetricsArrayTimeout        = "30s"
	powerMaxDefaultMetricsArrayCBThreshold    = int32(3)
	powerMaxDefaultMetricsArrayCBResetTimeout = "30s"
)

/*
CustomEnv - Custom environment variable

Usage: Contain dynamically configurable environment variables to be easily acccessible during substition.
*/
type CustomEnv struct {
	Name  string // Name of the environment variable
	Value string // Value of the environment variable
}

// MountCredentialsEnvs contains environment variables for mounting credentials
var (
	MountCredentialsEnvs = []CustomEnv{
		{Name: CSIPowerMaxSecretFilePath, Value: CSIPowerMaxSecretMountPath + "/" + CSIPowerMaxSecretName},
		{Name: CSIPowerMaxUseSecret, Value: "true"},
		{Name: CSIPowerMaxConfigPathKey, Value: CSIPowerMaxConfigPathValue},
	}

	MountCredentialsVolumeMounts = []CustomEnv{
		{Name: CSIPowerMaxSecretVolumeName, Value: CSIPowerMaxSecretMountPath},
		{Name: PowerMaxConfigParamsVolumeMount, Value: PowerMaxConfigParamsVolumeMount},
	}
)

// PrecheckPowerMax do input validation
func PrecheckPowerMax(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig operatorutils.OperatorConfig, ct client.Client) error {
	log := logger.GetLogger(ctx)

	version, err := operatorutils.GetVersion(ctx, cr, operatorConfig)
	if err != nil {
		return err
	}
	// Check if driver version is supported by doing a stat on a config file
	configFilePath := fmt.Sprintf("%s/driverconfig/powermax/%s/driver-config-params.yaml", operatorConfig.ConfigDirectory, version)
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Errorw("PreCheckPowerMax failed in version check", "Error", err.Error())
		return fmt.Errorf("%s %s not supported", csmv1.PowerMax, version)
	}

	secretName := cr.Name + "-creds"
	if cr.Spec.Driver.AuthSecret != "" {
		secretName = cr.Spec.Driver.AuthSecret
	}

	useReverseProxySecret := UseReverseProxySecret(cr)
	if useReverseProxySecret {
		log.Infof("[PrecheckPowerMax] Using Secret: %s", secretName)
	} else {
		log.Infof("[PrecheckPowerMax] Using ConfigMap: %s", secretName)
	}

	found := &corev1.Secret{}
	err = ct.Get(ctx, types.NamespacedName{Name: secretName, Namespace: cr.GetNamespace()}, found)
	if err != nil {
		log.Error(err, "Failed query for secret", secretName)
		if errors.IsNotFound(err) {
			return fmt.Errorf("failed to find secret %s", secretName)
		}
	}

	for i, mod := range cr.Spec.Modules {
		if mod.Name == csmv1.ReverseProxy {
			cr.Spec.Modules[i].Enabled = true
			cr.Spec.Modules[i].ForceRemoveModule = true
			break
		}
	}

	// Check for metrics TLS secret if metrics is enabled with TLS
	if isDriverMetricsTLSEnabled(*cr) {
		found := &corev1.Secret{}
		secretName := cr.Spec.Driver.Metrics.TLSCertSecret
		err := ct.Get(ctx, types.NamespacedName{Name: secretName, Namespace: cr.GetNamespace()}, found)
		if err != nil {
			log.Error(err, "Failed query for metrics TLS secret", secretName, "Namespace", cr.Namespace)
			if errors.IsNotFound(err) {
				return fmt.Errorf("failed to find metrics TLS secret %s", secretName)
			}
			return err
		}
	}

	log.Debugw("preCheck", "secrets", secretName)
	return nil
}

// UseReverseProxySecret checks if reverse proxy secret is configured in the CR
func UseReverseProxySecret(cr *csmv1.ContainerStorageModule) bool {
	useSecret := false

	if cr.Spec.Driver.Common == nil {
		return false
	}

	for _, env := range cr.Spec.Driver.Common.Envs {
		if env.Name == CSIPowerMaxUseSecret {
			ok, err := strconv.ParseBool(env.Value)
			if err != nil {
				log.Printf("Error parsing %s, %s. Using configMap solution", CSIPowerMaxUseSecret, err.Error())
				return false
			}
			useSecret = ok
		}
	}

	return useSecret
}

// ModifyPowermaxCR -
func ModifyPowermaxCR(yamlString string, cr csmv1.ContainerStorageModule, fileType string) string {
	// Parameters to initialise CR values
	managedArray := ""
	endpoint := ""
	debug := "false"
	portGroup := ""
	protocol := ""
	nodeTemplate := ""
	modifyHostname := "false"
	nodeTopology := "false"
	vsphereEnabled := "false"
	vspherePG := ""
	vsphereHostname := ""
	vsphereHost := ""
	nodeChap := "false"
	ctrlHealthMonitor := "false"
	nodeHealthMonitor := "false"
	storageCapacity := "true"
	maxVolumesPerNode := ""
	dynamicSGEnabled := "false"
	csiAddonsReplEnabled := "false"
	metroSiteFailureEnabled := "false"
	metroStateCheckTimeout := ""
	metroQueueWarningThreshold := ""
	metroQueueHardLimit := ""
	metroReconciliationBackoff := ""
	driverInstanceUID := ""
	capacityPollInterval := "5m"
	capacityThresholdFull := "100"
	fsckEnabled := GetDriverCommonEnv(cr, CsiFsCheckEnabled, "false")
	fsckMode := GetDriverCommonEnv(cr, CsiFsCheckMode, "checkOnly")

	spaceReclamationEnabled := GetDriverCommonEnv(cr, CsiSpaceReclamationEnabled, "false")
	spaceReclamationSchedule := GetDriverCommonEnv(cr, CsiSpaceReclamationSchedule, "")
	spaceReclamationMaxConcurrent := GetDriverCommonEnv(cr, CsiSpaceReclamationMaxConcurrent, "")
	spaceReclamationTimeOut := GetDriverCommonEnv(cr, CsiSpaceReclamationTimeOut, "")

	// Metrics configuration
	metricsEnabled := "false"
	metricsPort := "8443"
	metricsTLSCertFile := ""
	metricsTLSKeyFile := ""
	metricsCollectionInterval := powerMaxDefaultMetricsCollectionInterval
	metricsCollectionCacheTTL := powerMaxDefaultMetricsCollectionCacheTTL
	metricsArrayRateLimit := strconv.FormatInt(int64(powerMaxDefaultMetricsArrayRateLimit), 10)
	metricsArrayTimeout := powerMaxDefaultMetricsArrayTimeout
	metricsArrayCBThreshold := strconv.FormatInt(int64(powerMaxDefaultMetricsArrayCBThreshold), 10)
	metricsArrayCBResetTimeout := powerMaxDefaultMetricsArrayCBResetTimeout

	if cr.Spec.Driver.Metrics != nil {
		if cr.Spec.Driver.Metrics.Enabled {
			metricsEnabled = "true"
		}
		if cr.Spec.Driver.Metrics.Port != 0 {
			metricsPort = fmt.Sprintf("%d", cr.Spec.Driver.Metrics.Port)
		}
		if isDriverMetricsTLSEnabled(cr) {
			metricsTLSCertFile = "/etc/metrics-tls/tls.crt"
			metricsTLSKeyFile = "/etc/metrics-tls/tls.key"
		}

		// Parse collection configuration
		if cr.Spec.Driver.Metrics.Collection != nil {
			metricsCollectionInterval = metricsDurationOrDefault(
				cr.Spec.Driver.Metrics.Collection.Interval,
				powerMaxDefaultMetricsCollectionInterval,
			)
			metricsCollectionCacheTTL = metricsDurationOrDefault(
				cr.Spec.Driver.Metrics.Collection.CacheTTL,
				powerMaxDefaultMetricsCollectionCacheTTL,
			)
		}

		// Parse array configuration
		if cr.Spec.Driver.Metrics.Array != nil {
			metricsArrayRateLimit = metricsPositiveIntOrDefault(
				cr.Spec.Driver.Metrics.Array.RateLimit,
				powerMaxDefaultMetricsArrayRateLimit,
			)
			metricsArrayTimeout = metricsDurationOrDefault(
				cr.Spec.Driver.Metrics.Array.Timeout,
				powerMaxDefaultMetricsArrayTimeout,
			)
			if cr.Spec.Driver.Metrics.Array.CircuitBreaker != nil {
				metricsArrayCBThreshold = metricsPositiveIntOrDefault(
					cr.Spec.Driver.Metrics.Array.CircuitBreaker.Threshold,
					powerMaxDefaultMetricsArrayCBThreshold,
				)
				metricsArrayCBResetTimeout = metricsDurationOrDefault(
					cr.Spec.Driver.Metrics.Array.CircuitBreaker.ResetTimeout,
					powerMaxDefaultMetricsArrayCBResetTimeout,
				)
			}
		}
	}

	// #nosec G101 - False positives
	switch fileType {
	case "Node":
		if cr.Spec.Driver.Common != nil {
			for _, env := range cr.Spec.Driver.Common.Envs {
				if env.Name == "X_CSI_MANAGED_ARRAYS" {
					managedArray = env.Value
				}
				if env.Name == "X_CSI_POWERMAX_ENDPOINT" {
					endpoint = env.Value
				}
				if env.Name == "X_CSI_POWERMAX_DEBUG" {
					debug = env.Value
				}
				if env.Name == "X_CSI_POWERMAX_PORTGROUPS" {
					portGroup = env.Value
				}
				if env.Name == "X_CSI_TRANSPORT_PROTOCOL" {
					protocol = env.Value
				}
				if env.Name == "X_CSI_VSPHERE_ENABLED" {
					vsphereEnabled = env.Value
				}
				if env.Name == "X_CSI_VSPHERE_PORTGROUP" {
					vspherePG = env.Value
				}
				if env.Name == "X_CSI_VSPHERE_HOSTNAME" {
					vsphereHostname = env.Value
				}
				if env.Name == "X_CSI_VCENTER_HOST" {
					vsphereHost = env.Value
				}
				if env.Name == "X_CSI_VSPHERE_ENABLED" {
					vsphereEnabled = env.Value
				}
				if env.Name == "X_CSI_IG_MODIFY_HOSTNAME" {
					modifyHostname = env.Value
				}
				if env.Name == "X_CSI_IG_NODENAME_TEMPLATE" {
					nodeTemplate = env.Value
				}
				if env.Name == "X_CSI_DYNAMIC_SG_ENABLED" {
					dynamicSGEnabled = env.Value
				}
			}
		}
		if cr.Spec.Driver.Node != nil {
			for _, env := range cr.Spec.Driver.Node.Envs {
				if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
					nodeHealthMonitor = env.Value
				}
				if env.Name == "X_CSI_POWERMAX_ISCSI_ENABLE_CHAP" {
					nodeChap = env.Value
				}
				if env.Name == "X_CSI_TOPOLOGY_CONTROL_ENABLED" {
					nodeTopology = env.Value
				}
				if env.Name == "X_CSI_MAX_VOLUMES_PER_NODE" {
					maxVolumesPerNode = env.Value
				}
			}
		}
		if cr.Spec.Driver.MetroSiteFailureHandling != nil {
			if cr.Spec.Driver.MetroSiteFailureHandling.Enabled {
				metroSiteFailureEnabled = "true"
			} else {
				metroSiteFailureEnabled = "false"
			}
			if cr.Spec.Driver.MetroSiteFailureHandling.StateCheckTimeoutSeconds != nil {
				metroStateCheckTimeout = fmt.Sprintf("%d", *cr.Spec.Driver.MetroSiteFailureHandling.StateCheckTimeoutSeconds)
			}
			if cr.Spec.Driver.MetroSiteFailureHandling.QueueWarningThreshold != nil {
				metroQueueWarningThreshold = fmt.Sprintf("%d", *cr.Spec.Driver.MetroSiteFailureHandling.QueueWarningThreshold)
			}
			if cr.Spec.Driver.MetroSiteFailureHandling.QueueHardLimit != nil {
				metroQueueHardLimit = fmt.Sprintf("%d", *cr.Spec.Driver.MetroSiteFailureHandling.QueueHardLimit)
			}
			if cr.Spec.Driver.MetroSiteFailureHandling.ReconciliationBackoffSeconds != nil {
				metroReconciliationBackoff = fmt.Sprintf("%d", *cr.Spec.Driver.MetroSiteFailureHandling.ReconciliationBackoffSeconds)
			}
		}
		proxyTLSSecret := RevProxyTLSSecretDefaultName
		revProxy := cr.GetModule(csmv1.ReverseProxy)
		for _, component := range revProxy.Components {
			if component.Name == ReverseProxyServerComponent {
				for _, env := range component.Envs {
					if env.Name == "X_CSI_REVPROXY_TLS_SECRET" {
						proxyTLSSecret = env.Value
					}
				}
			}
		}

		yamlString = strings.ReplaceAll(yamlString, CSIPmaxManagedArray, managedArray)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxEndpoint, endpoint)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxDebug, debug)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxPortGroup, portGroup)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxProtocol, protocol)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxNodeTemplate, nodeTemplate)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxModifyHostname, modifyHostname)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxHealthMonitor, nodeHealthMonitor)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxTopology, nodeTopology)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxVsphere, vsphereEnabled)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxVspherePG, vspherePG)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxVsphereHostname, vsphereHostname)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxVsphereHost, vsphereHost)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxChap, nodeChap)
		yamlString = strings.ReplaceAll(yamlString, CsiPmaxMaxVolumesPerNode, maxVolumesPerNode)
		yamlString = strings.ReplaceAll(yamlString, ReverseProxyTLSSecret, proxyTLSSecret)
		yamlString = strings.ReplaceAll(yamlString, CSMNameSpace, cr.Namespace)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxDynamicSGEnabled, dynamicSGEnabled)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxMetroSiteFailureEnabled, metroSiteFailureEnabled)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxMetroStateCheckTimeout, metroStateCheckTimeout)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxMetroQueueWarningThreshold, metroQueueWarningThreshold)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxMetroQueueHardLimit, metroQueueHardLimit)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxMetroReconciliationBackoff, metroReconciliationBackoff)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsEnabled, metricsEnabled)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsPort, metricsPort)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsTLSCertFile, metricsTLSCertFile)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsTLSKeyFile, metricsTLSKeyFile)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsCollectionInterval, metricsCollectionInterval)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsCollectionCacheTTL, metricsCollectionCacheTTL)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayRateLimit, metricsArrayRateLimit)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayTimeout, metricsArrayTimeout)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayCBThreshold, metricsArrayCBThreshold)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayCBResetTimeout, metricsArrayCBResetTimeout)

		yamlString = SubstituteEnvVar(yamlString, CsiFsCheckEnabled, fsckEnabled)
		yamlString = SubstituteEnvVar(yamlString, CsiFsCheckMode, fsckMode)
		yamlString = SubstituteEnvVar(yamlString, CsiSpaceReclamationEnabled, spaceReclamationEnabled)
		yamlString = SubstituteEnvVar(yamlString, CsiSpaceReclamationSchedule, spaceReclamationSchedule)
		yamlString = SubstituteEnvVar(yamlString, CsiSpaceReclamationMaxConcurrent, spaceReclamationMaxConcurrent)
		yamlString = SubstituteEnvVar(yamlString, CsiSpaceReclamationTimeOut, spaceReclamationTimeOut)
	case "Controller":
		if cr.Spec.Driver.Common != nil {
			for _, env := range cr.Spec.Driver.Common.Envs {
				if env.Name == "X_CSI_MANAGED_ARRAYS" {
					managedArray = env.Value
				}
				if env.Name == "X_CSI_POWERMAX_ENDPOINT" {
					endpoint = env.Value
				}
				if env.Name == "X_CSI_POWERMAX_DEBUG" {
					debug = env.Value
				}
				if env.Name == "X_CSI_POWERMAX_PORTGROUPS" {
					portGroup = env.Value
				}
				if env.Name == "X_CSI_TRANSPORT_PROTOCOL" {
					protocol = env.Value
				}
				if env.Name == "X_CSI_VSPHERE_ENABLED" {
					vsphereEnabled = env.Value
				}
				if env.Name == "X_CSI_VSPHERE_PORTGROUP" {
					vspherePG = env.Value
				}
				if env.Name == "X_CSI_VSPHERE_HOSTNAME" {
					vsphereHostname = env.Value
				}
				if env.Name == "X_CSI_VCENTER_HOST" {
					vsphereHost = env.Value
				}
				if env.Name == "X_CSI_IG_MODIFY_HOSTNAME" {
					modifyHostname = env.Value
				}
				if env.Name == "X_CSI_IG_NODENAME_TEMPLATE" {
					nodeTemplate = env.Value
				}
				if env.Name == "X_CSI_DYNAMIC_SG_ENABLED" {
					dynamicSGEnabled = env.Value
				}
				if env.Name == "X_CSI_CSIADDONS_REPLICATION_ENABLED" {
					csiAddonsReplEnabled = env.Value
				}
				if env.Name == "X_CSI_POWERMAX_METRO_SITE_FAILURE_HANDLING_ENABLED" {
					metroSiteFailureEnabled = env.Value
				}
				if env.Name == "X_CSI_POWERMAX_METRO_STATE_CHECK_TIMEOUT" {
					metroStateCheckTimeout = env.Value
				}
				if env.Name == "X_CSI_POWERMAX_METRO_QUEUE_WARNING_THRESHOLD" {
					metroQueueWarningThreshold = env.Value
				}
				if env.Name == "X_CSI_POWERMAX_METRO_QUEUE_HARD_LIMIT" {
					metroQueueHardLimit = env.Value
				}
				if env.Name == "X_CSI_POWERMAX_METRO_RECONCILIATION_BACKOFF" {
					metroReconciliationBackoff = env.Value
				}
				if env.Name == "X_CSI_DRIVER_INSTANCE_UID" {
					driverInstanceUID = env.Value
				}
			}
		}

		// AC-001 fix: Read Metro site-failure handling configuration from CR field
		// This takes precedence over environment variables for Operator deployments
		if cr.Spec.Driver.MetroSiteFailureHandling != nil {
			if cr.Spec.Driver.MetroSiteFailureHandling.Enabled {
				metroSiteFailureEnabled = "true"
			} else {
				metroSiteFailureEnabled = "false"
			}
			if cr.Spec.Driver.MetroSiteFailureHandling.StateCheckTimeoutSeconds != nil {
				metroStateCheckTimeout = fmt.Sprintf("%d", *cr.Spec.Driver.MetroSiteFailureHandling.StateCheckTimeoutSeconds)
			}
			if cr.Spec.Driver.MetroSiteFailureHandling.QueueWarningThreshold != nil {
				metroQueueWarningThreshold = fmt.Sprintf("%d", *cr.Spec.Driver.MetroSiteFailureHandling.QueueWarningThreshold)
			}
			if cr.Spec.Driver.MetroSiteFailureHandling.QueueHardLimit != nil {
				metroQueueHardLimit = fmt.Sprintf("%d", *cr.Spec.Driver.MetroSiteFailureHandling.QueueHardLimit)
			}
			if cr.Spec.Driver.MetroSiteFailureHandling.ReconciliationBackoffSeconds != nil {
				metroReconciliationBackoff = fmt.Sprintf("%d", *cr.Spec.Driver.MetroSiteFailureHandling.ReconciliationBackoffSeconds)
			}
		}

		if cr.Spec.Driver.Controller != nil {
			for _, env := range cr.Spec.Driver.Controller.Envs {
				if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
					ctrlHealthMonitor = env.Value
				}
				if env.Name == "X_CSI_CAPACITY_POLL_INTERVAL" {
					capacityPollInterval = env.Value
				}
				if env.Name == "X_CSI_CAPACITY_THRESHOLD_FULL" {
					capacityThresholdFull = env.Value
				}
			}
		}

		proxyTLSSecret := RevProxyTLSSecretDefaultName
		revProxy := cr.GetModule(csmv1.ReverseProxy)
		for _, component := range revProxy.Components {
			if component.Name == ReverseProxyServerComponent {
				for _, env := range component.Envs {
					if env.Name == "X_CSI_REVPROXY_TLS_SECRET" {
						proxyTLSSecret = env.Value
					}
				}
			}
		}

		yamlString = strings.ReplaceAll(yamlString, CSIPmaxManagedArray, managedArray)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxEndpoint, endpoint)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxDebug, debug)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxPortGroup, portGroup)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxProtocol, protocol)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxNodeTemplate, nodeTemplate)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxModifyHostname, modifyHostname)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxHealthMonitor, ctrlHealthMonitor)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxTopology, nodeTopology)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxVsphere, vsphereEnabled)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxVspherePG, vspherePG)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxVsphereHostname, vsphereHostname)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxVsphereHost, vsphereHost)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxChap, nodeChap)
		yamlString = strings.ReplaceAll(yamlString, ReverseProxyTLSSecret, proxyTLSSecret)
		yamlString = strings.ReplaceAll(yamlString, CSMNameSpace, cr.Namespace)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxDynamicSGEnabled, dynamicSGEnabled)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsEnabled, metricsEnabled)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsPort, metricsPort)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsTLSCertFile, metricsTLSCertFile)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsTLSKeyFile, metricsTLSKeyFile)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsCollectionInterval, metricsCollectionInterval)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsCollectionCacheTTL, metricsCollectionCacheTTL)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayRateLimit, metricsArrayRateLimit)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayTimeout, metricsArrayTimeout)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayCBThreshold, metricsArrayCBThreshold)
		yamlString = strings.ReplaceAll(yamlString, constants.CsiMetricsArrayCBResetTimeout, metricsArrayCBResetTimeout)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxCSIAddonsReplEnabled, csiAddonsReplEnabled)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxMetroSiteFailureEnabled, metroSiteFailureEnabled)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxMetroStateCheckTimeout, metroStateCheckTimeout)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxMetroQueueWarningThreshold, metroQueueWarningThreshold)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxMetroQueueHardLimit, metroQueueHardLimit)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxMetroReconciliationBackoff, metroReconciliationBackoff)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxDriverInstanceUID, driverInstanceUID)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxCapacityPollInterval, capacityPollInterval)
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxCapacityThresholdFull, capacityThresholdFull)
	case "CSIDriverSpec":
		// Note: StorageCapacity is a bool with omitempty. Go's zero-value for bool is false,
		// so we cannot distinguish between "user omitted the field" and "user explicitly set false"
		// without changing the type to *bool. The chosen behavior is: default to "true" (matching
		// the pre-placeholder hardcoded template), and only override to "false" when the user sets
		// csiDriverSpec.storageCapacity: false explicitly. Users who set a csiDriverSpec block
		// without specifying storageCapacity must include storageCapacity: true to preserve the default.
		if cr.Spec.Driver.CSIDriverSpec != nil && !cr.Spec.Driver.CSIDriverSpec.StorageCapacity {
			storageCapacity = "false"
		}
		yamlString = strings.ReplaceAll(yamlString, CsiStorageCapacityEnabled, storageCapacity)
	}

	return yamlString
}

// GetProxyAuthTokenSecretName returns the auth token secret name from the CR env vars.
// Users set X_CSI_REVPROXY_AUTH_TOKEN_SECRET in common.envs to enable the feature.
func GetProxyAuthTokenSecretName(cr *csmv1.ContainerStorageModule) string {
	if cr.Spec.Driver.Common == nil {
		return ""
	}
	for _, env := range cr.Spec.Driver.Common.Envs {
		if env.Name == CSIPowerMaxProxyAuthTokenSecretName && env.Value != "" {
			return env.Value
		}
	}
	return ""
}

// DynamicallyMountPowermaxContent dynamically mounts PowerMax secret or configMap content
func DynamicallyMountPowermaxContent(configuration interface{}, cr csmv1.ContainerStorageModule) error {
	var podTemplate *acorev1.PodTemplateSpecApplyConfiguration
	switch configuration := configuration.(type) {
	case *v1.DeploymentApplyConfiguration:
		dp := configuration
		podTemplate = dp.Spec.Template
	case *v1.DaemonSetApplyConfiguration:
		ds := configuration
		podTemplate = ds.Spec.Template
	}

	if podTemplate == nil {
		return fmt.Errorf("invalid type passed through")
	}

	secretName := cr.Name + "-creds"
	if cr.Spec.Driver.AuthSecret != "" {
		secretName = cr.Spec.Driver.AuthSecret
	}

	if UseReverseProxySecret(&cr) {
		volumeName := CSIPowerMaxSecretVolumeName
		optional := false

		// Adding volume
		podTemplate.Spec.Volumes = append(podTemplate.Spec.Volumes,
			acorev1.VolumeApplyConfiguration{
				Name:                           &volumeName,
				VolumeSourceApplyConfiguration: acorev1.VolumeSourceApplyConfiguration{Secret: &acorev1.SecretVolumeSourceApplyConfiguration{SecretName: &secretName, Optional: &optional}},
			})

		// Adding volume mount for both the reverseproxy and driver
		for i, cnt := range podTemplate.Spec.Containers {
			if *cnt.Name == "driver" || *cnt.Name == "reverseproxy" || *cnt.Name == "karavi-metrics-powermax" {
				setPowermaxMountCredentialContent(&podTemplate.Spec.Containers[i])
			}
		}

		return nil
	}

	for i, cnt := range podTemplate.Spec.Containers {
		if *cnt.Name == "driver" {
			SetPowermaxConfigContent(&podTemplate.Spec.Containers[i], secretName)
			SetDriverMetrics(csmv1.PowerMax, cr, &podTemplate.Spec.Containers[i])
			break
		}
	}

	return nil
}

func setPowermaxMountCredentialContent(ct *acorev1.ContainerApplyConfiguration) {
	for _, mount := range MountCredentialsVolumeMounts {
		dynamicallyMountVolume(ct, acorev1.VolumeMountApplyConfiguration{
			Name:      &mount.Name,
			MountPath: &mount.Value,
		})
	}

	for _, env := range MountCredentialsEnvs {
		dynamicallyAddEnvironmentVariable(ct, acorev1.EnvVarApplyConfiguration{
			Name:  &env.Name,
			Value: &env.Value,
		})
	}
}

func dynamicallyMountVolume(ct *acorev1.ContainerApplyConfiguration, mount acorev1.VolumeMountApplyConfiguration) {
	contains := slices.ContainsFunc(
		ct.VolumeMounts,
		func(v acorev1.VolumeMountApplyConfiguration) bool { return *v.Name == *mount.Name },
	)

	if !contains {
		ct.VolumeMounts = append(ct.VolumeMounts, mount)
	}
}

// SetPowermaxConfigContent sets PowerMax config content from secret
func SetPowermaxConfigContent(ct *acorev1.ContainerApplyConfiguration, secretName string) {
	userNameVariable := "X_CSI_POWERMAX_USER"
	userNameKey := "username"
	userPasswordVariable := "X_CSI_POWERMAX_PASSWORD" // #nosec G101
	userPasswordKey := "password"
	dynamicallyAddEnvironmentVariable(ct, acorev1.EnvVarApplyConfiguration{
		Name: &userNameVariable,
		ValueFrom: &acorev1.EnvVarSourceApplyConfiguration{
			SecretKeyRef: &acorev1.SecretKeySelectorApplyConfiguration{
				Key: &userNameKey,
				LocalObjectReferenceApplyConfiguration: acorev1.LocalObjectReferenceApplyConfiguration{
					Name: &secretName,
				},
			},
		},
	})

	dynamicallyAddEnvironmentVariable(ct, acorev1.EnvVarApplyConfiguration{
		Name: &userPasswordVariable,
		ValueFrom: &acorev1.EnvVarSourceApplyConfiguration{
			SecretKeyRef: &acorev1.SecretKeySelectorApplyConfiguration{
				Key: &userPasswordKey,
				LocalObjectReferenceApplyConfiguration: acorev1.LocalObjectReferenceApplyConfiguration{
					Name: &secretName,
				},
			},
		},
	})
}

func dynamicallyAddEnvironmentVariable(ct *acorev1.ContainerApplyConfiguration, envVar acorev1.EnvVarApplyConfiguration) {
	contains := slices.ContainsFunc(
		ct.Env,
		func(v acorev1.EnvVarApplyConfiguration) bool { return *v.Name == *envVar.Name },
	)

	if !contains {
		ct.Env = append(ct.Env, envVar)
	}
}

func getApplyCertVolumePowermax(cr csmv1.ContainerStorageModule) (*acorev1.VolumeApplyConfiguration, error) {
	name := "certs"
	secretName := fmt.Sprintf("%s-%s", cr.Name, name)
	optional := true
	volume := &acorev1.VolumeApplyConfiguration{
		Name: &name,
		VolumeSourceApplyConfiguration: acorev1.VolumeSourceApplyConfiguration{
			Secret: &acorev1.SecretVolumeSourceApplyConfiguration{
				SecretName: &secretName,
				Optional:   &optional,
			},
		},
	}
	return volume, nil
}
