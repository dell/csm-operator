//  Copyright © 2022-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package steps

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StepDefinition - definition of a step
type StepDefinition struct {
	Handler reflect.Value
	Expr    *regexp.Regexp
}

// Runner -
type Runner struct {
	Definitions []StepDefinition
}

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

// StepRunnerInit -
func StepRunnerInit(runner *Runner, ctrlClient client.Client, clientSet *kubernetes.Clientset) {
	step := Step{
		ctrlClient: ctrlClient,
		clientSet:  clientSet,
	}
	runner.addStep(`^Given an environment with k8s or openshift, and CSM operator installed$`, step.validateTestEnvironment)
	runner.addStep(`^Given an environment with k8s or openshift, and CSM operator is not installed$`, step.validateKubernetesEnvironment)
	runner.addStep(`^Install \[([^"]*)\]$`, step.installThirdPartyModule)
	runner.addStep(`^Uninstall \[([^"]*)\]$`, step.uninstallThirdPartyModule)
	runner.addStep(`^Apply custom resource \[(\d+)\]$`, step.applyCustomResource)
	runner.addStep(`^Apply Authorization with Conjur custom resource \[(\d+)\]$`, step.applyAuthorizationConjur)
	runner.addStep(`^Validate \[(\d+)\] CSM has forceRemoveDriver set to true$`, step.validateForceRemoveDriverEnabled)
	runner.addStep(`^Validate \[(\d+)\] CSM has forceRemoveDriver set to false$`, step.validateForceRemoveDriverDisabled)
	runner.addStep(`^Upgrade from custom resource \[(\d+)\] to \[(\d+)\]$`, step.upgradeCustomResource)
	runner.addStep(`^Validate custom resource \[(\d+)\]$`, step.validateCustomResourceStatus)
	runner.addStep(`^Validate deployment from CR \[(\d+)\] has argument \[([^"]*)\] in container \[([^"]*)\]$`, step.validateContainerArg)
	runner.addStep(`^Validate deployment from CR \[(\d+)\] has image \[([^"]*)\] in container \[([^"]*)\]$`, step.validateDeploymentContainerImage)
	runner.addStep(`^Validate deployment from CR \[(\d+)\] has image containing \[([^"]*)\] in container \[([^"]*)\]$`, step.validateDeploymentContainerImageContains)
	runner.addStep(`^Validate daemonset from CR \[(\d+)\] has image \[([^"]*)\] in container \[([^"]*)\]$`, step.validateDaemonSetContainerImage)
	runner.addStep(`^Validate daemonset from CR \[(\d+)\] has image containing \[([^"]*)\] in container \[([^"]*)\]$`, step.validateDaemonSetContainerImageContains)
	runner.addStep(`^Validate \[([^"]*)\] driver from CR \[(\d+)\] is installed$`, step.validateDriverInstalled)
	runner.addStep(`^Validate \[([^"]*)\] driver spec from CR \[(\d+)\]$`, step.validateMinimalCSMDriverSpec)
	runner.addStep(`^Validate \[([^"]*)\] driver from CR \[(\d+)\] is not installed$`, step.validateDriverNotInstalled)

	runner.addStep(`^Run custom test$`, step.runCustomTest)         // legacy support - original e2e was designed only to run ONE custom test
	runner.addStep(`^Run \[([^"]*)\]$`, step.runCustomTestSelector) // support for multiple custom tests
	runner.addStep(`^Enable forceRemoveDriver on CR \[(\d+)\]$`, step.enableForceRemoveDriver)
	runner.addStep(`^Enable forceRemoveModule on CR \[(\d+)\]$`, step.enableForceRemoveModule)
	runner.addStep(`^Delete custom resource \[(\d+)\]$`, step.deleteCustomResource)

	runner.addStep(`^Validate \[([^"]*)\] module from CR \[(\d+)\] is installed$`, step.validateModuleInstalled)
	runner.addStep(`^Validate \[([^"]*)\] module from CR \[(\d+)\] is not installed$`, step.validateModuleNotInstalled)
	runner.addStep(`^Validate \[([^"]*)\] module metrics from CR \[(\d+)\] is installed$`, step.validateModuleMetricsInstalled)
	runner.addStep(`^Validate csi-addons sidecar is installed in CR \[(\d+)\]$`, step.validateCSIAddonsSidecarInstalled)
	runner.addStep(`^Validate \[([^"]*)\] module pods from CR \[(\d+)\] is not installed$`, step.validateAuthorizationPodsNotInstalled)

	runner.addStep(`^Enable \[([^"]*)\] module from CR \[(\d+)\]$`, step.enableModule)
	runner.addStep(`^Disable \[([^"]*)\] module from CR \[(\d+)\]$`, step.disableModule)

	runner.addStep(`^(Enable|Disable) healthmonitor from CR \[(\d+)\]$`, step.configureHealthMonitor)

	runner.addStep(`^Set \[([^"]*)\] node label$`, step.setNodeLabel)
	runner.addStep(`^Remove \[([^"]*)\] node label$`, step.removeNodeLabel)

	runner.addStep(`^Set secret for driver from CR \[(\d+)\] to \[([^"]*)\]$`, step.setDriverSecret)
	runner.addStep(`^Create Secret with template \[([^"]*)\] name \[([^"]*)\] in namespace \[([^"]*)\] for \[([^"]*)\]`, step.setUpSecret)
	runner.addStep(`^Create Secret from file \[([^"]*)\] name \[([^"]*)\] in namespace \[([^"]*)\] for \[([^"]*)\]`, step.setUpSecretFromFile)
	runner.addStep(`^Create Secret from template \[([^"]*)\] as field \[([^"]*)\] named \[([^"]*)\] in namespace \[([^"]*)\] for \[([^"]*)\]`, step.setUpSecretFromTemplateWithFieldName)
	runner.addStep(`^Apply rendered YAML \[([^"]*)\] in namespace \[([^"]*)\] for \[([^"]*)\]`, step.applyRenderedYAML)
	runner.addStep(`^Generate and Create SFTP Secrets from template \[([^"]*)\] private-secret \[([^"]*)\] public-secret \[([^"]*)\] in namespace \[([^"]*)\] for \[([^"]*)\]$`, step.generateAndCreateSftpSecrets)
	runner.addStep(`^Configure Powerflex SFTP from template \[([^"]*)\] private-secret \[([^"]*)\] public-secret \[([^"]*)\] in namespace \[([^"]*)\] for \[([^"]*)\] CR \[(\d+)\]$`, step.configurePowerflexSFTP)
	runner.addStep(`^Create ConfigMap with template \[([^"]*)\] name \[([^"]*)\] in namespace \[([^"]*)\] for \[([^"]*)\]`, step.setUpConfigMap)
	runner.addStep(`^Create resource with template \[([^\]]*)\] in namespace \[([^\]]*)\]$`, step.createResourceInNamespace)
	runner.addStep(`^Create resource with template \[([^\]]*)\] in namespace \[([^\]]*)\] for \[([^\]]*)\]$`, step.createResourceInNamespaceWithType)
	runner.addStep(`^Create resource with template \[([^"]*)\] for \[([^"]*)\]`, step.createResource)
	runner.addStep(`^Create StorageClass with template \[([^"]*)\] for \[([^"]*)\]`, step.setUpStorageClass)
	runner.addStep(`^Create \[([^"]*)\] prerequisites from CR \[(\d+)\]$`, step.createPrereqs)
	runner.addStep(`^Set up ephemeral volume properties \[([^"]*)\] for \[([^"]*)\]`, step.setupEphemeralVolumeProperties)

	// Configure authorization-proxy-server for [powerflex]
	runner.addStep(`^Configure authorization-proxy-server with template \[([^"]*)\] for \[([^"]*)\] for CR \[(\d+)\]$`, step.configureAuthorizationProxyServer)
	// Authorization Proxy Server V2 additional steps
	runner.addStep(`^Install Authorization CRDs \[(\d+)\]$`, step.createCustomResourceDefinition)
	runner.addStep(`^Validate \[([^"]*)\] CRD for Authorization is installed$`, step.validateCustomResourceDefinition)
	runner.addStep(`^Delete Authorization CRs for \[([^"]*)\]$`, step.deleteAuthorizationCRs)
	// Rollback/Upgrade integration test steps
	runner.addStep(`^Validate pre-upgrade snapshot exists on CR \[(\d+)\]$`, step.validatePreUpgradeSnapshot)
	runner.addStep(`^Validate snapshot deleted on CR \[(\d+)\]$`, step.validateSnapshotDeleted)
	runner.addStep(`^Validate rollback completed on CR \[(\d+)\]$`, step.validateRollbackCompleted)
	runner.addStep(`^Validate rollback completed on CR \[(\d+)\] with version \[([^"]*)\]$`, step.validateRollbackCompletedWithVersion)
	// Auto-upgrade steps
	runner.addStep(`^Set upgrade policy to \[([^"]*)\] on CR \[(\d+)\]$`, step.setUpgradePolicy)
	runner.addStep(`^Set upgrade policy to \[([^"]*)\] in CR spec \[(\d+)\]$`, step.setUpgradePolicyInSpec)
	runner.addStep(`^Validate auto-upgrade completed on CR \[(\d+)\] with version \[([^"]*)\]$`, step.validateAutoUpgradeCompleted)
	runner.addStep(`^Delete Authorization CRDs \[(\d+)\]$`, step.deleteCustomResourceDefinition)
	runner.addStep(`^Remove finalizers from authorization CRs in namespace \[([^"]*)\]$`, step.removeAuthorizationFinalizers)
	runner.addStep(`^Set up reverse proxy tls secret namespace \[([^"]*)\]`, step.setUpReverseProxy)
	runner.addStep(`^Set up reverse proxy tls secret with SAN namespace \[([^"]*)\]`, step.setUpTLSSecretWithSAN)
	runner.addStep(`^Set up PowerFlex metrics TLS secret namespace \[([^"]*)\]`, step.setUpMetricsTLSSecret)
	runner.addStep(`^Set up Authorization metrics TLS secret namespace \[([^"]*)\]`, step.setUpAuthorizationMetricsTLSSecret)
	runner.addStep(`^Set up PowerScale metrics TLS secret namespace \[([^"]*)\]`, step.setUpPowerScaleMetricsTLSSecret)
	runner.addStep(`^Set up PowerScale Observability TLS secret namespace \[([^"]*)\]`, step.setUpPowerScaleObservabilityTLSSecret)
	runner.addStep(`^Set up PowerStore metrics TLS secret namespace \[([^"]*)\]`, step.setUpPowerStoreMetricsTLSSecret)
	runner.addStep(`^Set up PowerMax metrics TLS secret namespace \[([^"]*)\]`, step.setUpPowerMaxMetricsTLSSecret)
	runner.addStep(`^Set up PowerFlex replication metrics TLS secret namespace \[([^"]*)\]`, step.setUpReplicationMetricsTLSSecret)
	runner.addStep(`^Set up PowerStore replication metrics TLS secret namespace \[([^"]*)\]`, step.setUpPowerStoreReplicationMetricsTLSSecret)
	runner.addStep(`^Set up PowerMax replication metrics TLS secret namespace \[([^"]*)\]`, step.setUpPowerMaxReplicationMetricsTLSSecret)
	runner.addStep(`^Set up PowerScale replication metrics TLS secret namespace \[([^"]*)\]`, step.setUpPowerScaleReplicationMetricsTLSSecret)
	runner.addStep(`^Set up PowerFlex observability metrics TLS secret namespace \[([^"]*)\]$`, step.setUpPowerFlexObsMetricsTLSSecret)
	runner.addStep(`^Enable \[metrics\] in CR \[(\d+)\]$`, step.enableMetrics)
	runner.addStep(`^Disable \[metrics\] in CR \[(\d+)\]$`, step.disableMetrics)
	runner.addStep(`^Validate \[metrics\] endpoints are exposed in driver for resource \[(\d+)\]$`, step.validateMetricsEndpoints)
	runner.addStep(`^Restore ConfigMap$`, step.restoreConfigMap)
	runner.addStep(`^Delete ConfigMap$`, step.deleteConfigMap)

	// Environment variables management steps
	runner.addStep(`^Validate \[(node|controller)\] \[([^"]*)\] env \[([^"]*)\] is \[([^"]*)\] in driver for resource \[(\d+)\]$`, step.validateEnvInDriverPod)
	runner.addStep(`^Validate \[(common|node|controller)\] env \[([^"]*)\] is \[([^"]*)\] in CSM CR for resource \[(\d+)\]$`, step.validateEnvInCSMCR)
	runner.addStep(`^Set \[(common|node|controller)\] env \[([^"]*)\] to \[([^"]*)\] in resource \[(\d+)\]$`, step.setEnvInSpec)

	// Pre-apply CR modification steps
	runner.addStep(`^Set forceRemoveDriver to \[([^"]*)\] in CR spec \[(\d+)\]$`, step.setForceRemoveDriverInSpec)
	runner.addStep(`^Enable \[([^"]*)\] module in CR spec \[(\d+)\]$`, step.enableModuleInSpec)
	runner.addStep(`^Set \[(common|node|controller)\] env \[([^"]*)\] from env \[([^"]*)\] in resource \[(\d+)\]$`, step.setEnvFromEnvVarInSpec)
	runner.addStep(`^Set driver image to \[([^"]*)\] in CR spec \[(\d+)\]$`, step.setDriverImage)
	runner.addStep(`^Set replicas to \[(\d+)\] in CR spec \[(\d+)\]$`, step.setReplicasInSpec)
	runner.addStep(`^Set metadata name \[([^"]*)\] namespace \[([^"]*)\] in CR spec \[(\d+)\]$`, step.setMetadataInSpec)
	runner.addStep(`^Set imagePullPolicy to \[([^"]*)\] in CR spec \[(\d+)\]$`, step.setImagePullPolicyInSpec)
	runner.addStep(`^Set \[([^"]*)\] component \[([^"]*)\] image to \[([^"]*)\] in CR spec \[(\d+)\]$`, step.setModuleComponentImageInSpec)
	runner.addStep(`^Set \[([^"]*)\] component \[([^"]*)\] env \[([^"]*)\] to \[([^"]*)\] in CR spec \[(\d+)\]$`, step.setModuleComponentEnvInSpec)
	runner.addStep(`^Set \[([^"]*)\] component \[([^"]*)\] enabled \[([^"]*)\] in CR spec \[(\d+)\]$`, step.setModuleComponentEnabledInSpec)
	runner.addStep(`^Set \[([^"]*)\] module configVersion to \[([^"]*)\] in CR spec \[(\d+)\]$`, step.setModuleConfigVersionInSpec)
	runner.addStep(`^Set driver configVersion to \[([^"]*)\] in CR spec \[(\d+)\]$`, step.setDriverConfigVersionInSpec)
	runner.addStep(`^Set spec version to \[([^"]*)\] in CR spec \[(\d+)\]$`, step.setSpecVersionInSpec)
	runner.addStep(`^Remove \[([^"]*)\] from CR spec \[(\d+)\]$`, step.removeFieldFromSpec)
	runner.addStep(`^Set customRegistry to \[([^"]*)\] in CR spec \[(\d+)\]$`, step.setCustomRegistryInSpec)

	// Failure grace period and CR failed state validation
	runner.addStep(`^Delete all CSM from namespace \[([^"]*)\]$`, step.deleteAllCSMFromNamespace)
	runner.addStep(`^Set operator failure grace period to \[([^"]*)\]$`, step.setOperatorFailureGracePeriod)
	runner.addStep(`^Validate custom resource \[(\d+)\] is failed$`, step.validateCustomResourceFailed)
	runner.addStep(`^Set ConfigMap image \[([^"]*)\] to \[([^"]*)\]$`, step.setConfigMapImage)
	runner.addStep(`^Copy Secret \[([^"]*)\] from namespace \[([^"]*)\] to namespace \[([^"]*)\]$`, step.copySecret)

	// Metro Snapshot Restore E2E Test Steps
	runner.addStep(`^Create metro PVC with template \[([^"]*)\] in namespace \[([^"]*)\]$`, step.createMetroPVC)
	runner.addStep(`^Wait for PVC \[([^"]*)\] to be bound in namespace \[([^"]*)\]$`, step.waitForPVCBound)
	runner.addStep(`^Validate PVC \[([^"]*)\] is metro in namespace \[([^"]*)\]$`, step.validatePVCIsMetro)
	runner.addStep(`^Validate PVC \[([^"]*)\] is not metro in namespace \[([^"]*)\]$`, step.validatePVCIsNotMetro)
	runner.addStep(`^Create VolumeSnapshot with template \[([^"]*)\] in namespace \[([^"]*)\]$`, step.createVolumeSnapshot)
	runner.addStep(`^Wait for VolumeSnapshot \[([^"]*)\] to be ready in namespace \[([^"]*)\]$`, step.waitForSnapshotReady)
	runner.addStep(`^Create pod with template \[([^"]*)\] in namespace \[([^"]*)\]$`, step.createPod)
	runner.addStep(`^Wait for pod \[([^"]*)\] to be running in namespace \[([^"]*)\]$`, step.waitForPodRunning)
	runner.addStep(`^Write data \[([^"]*)\] to file \[([^"]*)\] in pod \[([^"]*)\] namespace \[([^"]*)\]$`, step.writeDataToPod)
	runner.addStep(`^Sync data in pod \[([^"]*)\] namespace \[([^"]*)\]$`, step.syncDataInPod)
	runner.addStep(`^Verify data \[([^"]*)\] in file \[([^"]*)\] in pod \[([^"]*)\] namespace \[([^"]*)\]$`, step.verifyDataInPod)
	runner.addStep(`^Validate PVC \[([^"]*)\] is pending in namespace \[([^"]*)\]$`, step.validatePVCPending)
	runner.addStep(`^Validate PVC \[([^"]*)\] event contains \[([^"]*)\] in namespace \[([^"]*)\]$`, step.validatePVCEventContains)
	runner.addStep(`^Delete pod \[([^"]*)\] in namespace \[([^"]*)\]$`, step.deletePod)
	runner.addStep(`^Delete PVC \[([^"]*)\] in namespace \[([^"]*)\]$`, step.deletePVC)
	runner.addStep(`^Delete resource \[([^"]*)\] in namespace \[([^"]*)\]$`, step.deleteResourceFromTemplate)
	runner.addStep(`^Delete VolumeSnapshot \[([^"]*)\] in namespace \[([^"]*)\]$`, step.deleteVolumeSnapshot)
	runner.addStep(`^Create VolumeSnapshotClass with template \[([^"]*)\]$`, step.createVolumeSnapshotClass)
	runner.addStep(`^Delete VolumeSnapshotClass \[([^"]*)\]$`, step.deleteVolumeSnapshotClass)
	runner.addStep(`^Wait for pods to be deleted in namespace \[([^"]*)\]$`, step.waitForPodsDeleted)

	// Resiliency metrics validation steps
	runner.addStep(`^Validate \[resiliency-metrics\] Service is created for resource \[(\d+)\]$`, step.validateResiliencyMetricsService)
	runner.addStep(`^Validate \[resiliency-metrics\] ServiceMonitor is created for resource \[(\d+)\]$`, step.validateResiliencyServiceMonitor)
	runner.addStep(`^Validate \[resiliency-metrics\] PodMonitor is created for resource \[(\d+)\]$`, step.validateResiliencyPodMonitor)
	runner.addStep(`^Validate \[res-metrics\] port is exposed in podmon container for resource \[(\d+)\]$`, step.validateResiliencyMetricsPort)
	runner.addStep(`^Validate \[resiliency-metrics\] TLS volume is mounted in podmon container for resource \[(\d+)\]$`, step.validateResiliencyMetricsTLSVolumeMount)
	runner.addStep(`^Validate \[resiliency-metrics\] ServiceMonitor has TLS configuration for resource \[(\d+)\]$`, step.validateResiliencyMetricsServiceMonitorTLS)
	runner.addStep(`^Validate \[resiliency-metrics\] PodMonitor has TLS configuration for resource \[(\d+)\]$`, step.validateResiliencyMetricsPodMonitorTLS)
	runner.addStep(`^Validate resiliency metrics TLS environment variables for resource \[(\d+)\]$`, step.validateResiliencyMetricsTLSEnvVars)

	// Replication metrics validation steps
	runner.addStep(`^Validate \[replication-metrics\] Service is created for resource \[(\d+)\]$`, step.validateReplicationMetricsService)
	runner.addStep(`^Validate \[replication-metrics\] ServiceMonitor is created for resource \[(\d+)\]$`, step.validateReplicationMetricsServiceMonitor)
	runner.addStep(`^Validate \[replication-metrics\] PodMonitor is created for resource \[(\d+)\]$`, step.validateReplicationMetricsPodMonitor)
	runner.addStep(`^Validate \[replication-metrics\] port is exposed in dell-csi-replicator container for resource \[(\d+)\]$`, step.validateReplicationMetricsPort)
	runner.addStep(`^Validate \[replication-metrics\] TLS volume is mounted in dell-csi-replicator container for resource \[(\d+)\]$`, step.validateReplicationMetricsTLSVolumeMount)
	runner.addStep(`^Validate \[replication-metrics\] ServiceMonitor has TLS configuration for resource \[(\d+)\]$`, step.validateReplicationMetricsServiceMonitorTLS)
	runner.addStep(`^Validate \[replication-metrics\] PodMonitor has TLS configuration for resource \[(\d+)\]$`, step.validateReplicationMetricsPodMonitorTLS)

	runner.addStep(`^Validate \[authorization-proxy-server-metrics\] Service is created for resource \[(\d+)\]$`, step.validateAuthorizationMetricsService)
	runner.addStep(`^Validate \[authorization-proxy-server-metrics-monitor\] ServiceMonitor is created for resource \[(\d+)\]$`, step.validateAuthorizationMetricsServiceMonitor)
	runner.addStep(`^Validate \[authorization-proxy-server-metrics\] port is exposed in proxy-server container for resource \[(\d+)\]$`, step.validateAuthorizationMetricsPort)
	runner.addStep(`^Validate \[authorization-proxy-server-metrics-monitor\] ServiceMonitor uses HTTPS for resource \[(\d+)\]$`, step.validateAuthorizationMetricsServiceMonitorHTTPS)

	// CSM Authorization PrometheusRule validation steps
	runner.addStep(`^Validate \[[^\]]*\] Authorization PrometheusRule is created for resource \[(\d+)\]$`, step.validateAuthorizationPrometheusRuleCreated)
	runner.addStep(`^Validate \[[^\]]*\] Authorization PrometheusRule is absent for resource \[(\d+)\]$`, step.validateAuthorizationPrometheusRuleAbsent)
	runner.addStep(`^Validate \[[^\]]*\] Authorization PrometheusRule contains (\d+) alert rules for resource \[(\d+)\]$`, step.validateAuthorizationPrometheusRuleAlertCount)
	runner.addStep(`^Validate \[[^\]]*\] Authorization PrometheusRule has CSM CR owner reference for resource \[(\d+)\]$`, step.validateAuthorizationPrometheusRuleOwnerReference)
	runner.addStep(`^Validate \[[^\]]*\] Authorization PrometheusRule thresholds match CR spec for resource \[(\d+)\]$`, step.validateAuthorizationPrometheusRuleThresholds)
	runner.addStep(`^Enable \[authorization-proxy-server\] PrometheusRule in CR \[(\d+)\]$`, step.enableAuthorizationPrometheusRule)
	runner.addStep(`^Disable \[authorization-proxy-server\] PrometheusRule in CR \[(\d+)\]$`, step.disableAuthorizationPrometheusRule)
	runner.addStep(`^Enable \[authorization-proxy-server\] metrics in CR \[(\d+)\]$`, step.enableAuthorizationMetrics)
	runner.addStep(`^Disable \[authorization-proxy-server\] metrics in CR \[(\d+)\]$`, step.disableAuthorizationMetrics)

	// Observability self-metrics validation steps (PowerScale)
	runner.addStep(`^Validate \[karavi-metrics-powerscale\] Service is created for resource \[(\d+)\]$`, step.validateObsMetricsService)
	runner.addStep(`^Validate \[karavi-metrics-powerscale-obs-monitor\] ServiceMonitor is created for resource \[(\d+)\]$`, step.validateObsMetricsServiceMonitor)
	runner.addStep(`^Validate \[karavi-metrics-powerscale\] port is exposed in karavi-metrics-powerscale container for resource \[(\d+)\]$`, step.validateObsMetricsPort)
	runner.addStep(`^Validate \[karavi-metrics-powerscale\] port 8443 is exposed in karavi-metrics-powerscale container for resource \[(\d+)\]$`, step.validateObsMetricsPort)
	runner.addStep(`^Validate \[karavi-metrics-powerscale-obs-monitor\] ServiceMonitor uses HTTPS for resource \[(\d+)\]$`, step.validateObsMetricsServiceMonitorHTTPS)
	runner.addStep(`^Validate \[karavi-metrics-powerscale\] ConfigMap reflects CR configuration for resource \[(\d+)\]$`, step.validateObsMetricsConfigMap)

	// Observability self-metrics validation steps (PowerStore)
	runner.addStep(`^Validate \[karavi-metrics-powerstore\] Service is created for resource \[(\d+)\]$`, step.validatePowerStoreObsMetricsService)
	runner.addStep(`^Validate \[karavi-metrics-powerstore-obs-monitor\] ServiceMonitor is created for resource \[(\d+)\]$`, step.validatePowerStoreObsMetricsServiceMonitor)
	runner.addStep(`^Validate \[karavi-metrics-powerstore\] port is exposed in karavi-metrics-powerstore container for resource \[(\d+)\]$`, step.validatePowerStoreObsMetricsPort)
	runner.addStep(`^Validate \[karavi-metrics-powerstore-obs-monitor\] ServiceMonitor uses HTTPS for resource \[(\d+)\]$`, step.validatePowerStoreObsMetricsServiceMonitorHTTPS)
	runner.addStep(`^Validate \[karavi-metrics-powerstore\] ConfigMap reflects CR configuration for resource \[(\d+)\]$`, step.validatePowerStoreObsMetricsConfigMap)

	// Observability self-metrics validation steps (PowerMax)
	runner.addStep(`^Validate \[karavi-metrics-powermax\] Service is created for resource \[(\d+)\]$`, step.validatePowerMaxObsMetricsService)
	runner.addStep(`^Validate \[karavi-metrics-powermax-obs-monitor\] ServiceMonitor is created for resource \[(\d+)\]$`, step.validatePowerMaxObsMetricsServiceMonitor)
	runner.addStep(`^Validate \[karavi-metrics-powermax\] port is exposed in karavi-metrics-powermax container for resource \[(\d+)\]$`, step.validatePowerMaxObsMetricsPort)
	runner.addStep(`^Validate \[karavi-metrics-powermax-obs-monitor\] ServiceMonitor uses HTTPS for resource \[(\d+)\]$`, step.validatePowerMaxObsMetricsServiceMonitorHTTPS)
	runner.addStep(`^Validate \[karavi-metrics-powermax\] ConfigMap reflects CR configuration for resource \[(\d+)\]$`, step.validatePowerMaxObsMetricsConfigMap)

	// Observability self-metrics validation steps (PowerFlex)
	runner.addStep(`^Validate \[karavi-metrics-powerflex\] Service is created for resource \[(\d+)\]$`, step.validatePowerFlexObsMetricsService)
	runner.addStep(`^Validate \[karavi-metrics-powerflex-obs-monitor\] ServiceMonitor is created for resource \[(\d+)\]$`, step.validatePowerFlexObsMetricsServiceMonitor)
	runner.addStep(`^Validate \[karavi-metrics-powerflex\] port is exposed in karavi-metrics-powerflex container for resource \[(\d+)\]$`, step.validatePowerFlexObsMetricsPort)
	runner.addStep(`^Validate \[karavi-metrics-powerflex\] ConfigMap reflects CR configuration for resource \[(\d+)\]$`, step.validatePowerFlexObsMetricsConfigMap)
	runner.addStep(`^Validate \[karavi-metrics-powerflex-obs-monitor\] ServiceMonitor uses HTTPS for resource \[(\d+)\]$`, step.validatePowerFlexObsMetricsServiceMonitorHTTPS)
	runner.addStep(`^Validate \[karavi-metrics-powerflex\] TLS volume is mounted in karavi-metrics-powerflex container for resource \[(\d+)\]$`, step.validatePowerFlexObsMetricsTLSVolumeMount)

	// Driver metrics validation steps
	runner.addStep(`^Validate \[[^\]]*\] driver metrics ServiceMonitor is created for resource \[(\d+)\]$`, step.validateDriverMetricsServiceMonitor)
	runner.addStep(`^Validate \[[^\]]*\] driver metrics PodMonitor is created for resource \[(\d+)\]$`, step.validateDriverMetricsPodMonitor)

	// Driver metrics PrometheusRule alerting steps
	runner.addStep(`^Validate \[[^\]]*\] driver metrics PrometheusRule is created for resource \[(\d+)\]$`, step.validateDriverMetricsPrometheusRule)
	runner.addStep(`^Validate \[[^\]]*\] driver metrics PrometheusRule is absent for resource \[(\d+)\]$`, step.validateDriverMetricsPrometheusRuleAbsent)
	runner.addStep(`^Validate \[[^\]]*\] driver metrics PrometheusRule contains alert \[([^\]]+)\] for resource \[(\d+)\]$`, step.validateDriverPrometheusRuleContainsAlert)
	runner.addStep(`^Enable \[prometheusRule\] in CR \[(\d+)\]$`, step.enablePrometheusRuleInCR)
	runner.addStep(`^Disable \[prometheusRule\] in CR \[(\d+)\]$`, step.disablePrometheusRuleInCR)

	runner.addStep(`^Validate \[replication\] PrometheusRule is created for resource \[(\d+)\]$`, step.validateReplicationPrometheusRuleCreated)
	runner.addStep(`^Validate \[replication\] PrometheusRule is absent for resource \[(\d+)\]$`, step.validateReplicationPrometheusRuleAbsent)
	runner.addStep(`^Validate \[replication\] PrometheusRule contains alert \[([^\]]+)\] for resource \[(\d+)\]$`, step.validateReplicationPrometheusRuleContainsAlert)
	runner.addStep(`^Enable \[replication\] PrometheusRule in CR \[(\d+)\]$`, step.enableReplicationPrometheusRule)
	runner.addStep(`^Disable \[replication\] PrometheusRule in CR \[(\d+)\]$`, step.disableReplicationPrometheusRule)

	// Observability module PrometheusRule alerting steps
	runner.addStep(`^Validate \[observability\] PrometheusRule is created for resource \[(\d+)\]$`, step.validateObservabilityPrometheusRule)
	runner.addStep(`^Validate \[observability\] PrometheusRule is absent for resource \[(\d+)\]$`, step.validateObservabilityPrometheusRuleAbsent)
	runner.addStep(`^Enable \[observability prometheusRule\] in CR \[(\d+)\]$`, step.enableObservabilityPrometheusRuleInCR)
	runner.addStep(`^Disable \[observability prometheusRule\] in CR \[(\d+)\]$`, step.disableObservabilityPrometheusRuleInCR)
	runner.addStep(`^Enable \[observability module\] from CR \[(\d+)\]$`, step.enableObservabilityModuleInCR)

	// Resiliency module PrometheusRule alerting steps
	runner.addStep(`^Validate \[resiliency\] PrometheusRule is created for resource \[(\d+)\]$`, step.validateResiliencyPrometheusRule)
	runner.addStep(`^Validate \[resiliency\] PrometheusRule is absent for resource \[(\d+)\]$`, step.validateResiliencyPrometheusRuleAbsent)
	runner.addStep(`^Enable \[resiliency prometheusRule\] in CR \[(\d+)\]$`, step.enableResiliencyPrometheusRuleInCR)
	runner.addStep(`^Disable \[resiliency prometheusRule\] in CR \[(\d+)\]$`, step.disableResiliencyPrometheusRuleInCR)

	// mTLS NFS Transport step mappings (ER-99506)

	// mTLS NFS Transport step mappings (ER-K8S-BR99506-001-powerscale-mtls-nfs-transport)

	runner.addStep(`^Validate mTLS environment variables are set for resource \[(\d+)\]$`, step.validateMTLSEnvVars)
	runner.addStep(`^Validate mTLS mount in Pod \[([^\]]+)\] in namespace \[([^\]]+)\]$`, step.validateMTLSMount)
	runner.addStep(`^Validate mTLS mount options for Pod \[([^\]]+)\] in namespace \[([^\]]+)\]$`, step.validateMTLSMountOptions)
	runner.addStep(`^Validate StorageClass FQDN precedence for Pod \[([^\]]+)\] in namespace \[([^\]]+)\]$`, step.validateStorageClassFQDNPrecedence)
	runner.addStep(`^Validate FQDN fallback precedence for Pod \[([^\]]+)\] in namespace \[([^\]]+)\]$`, step.validateFQDNFallbackPrecedence)
	runner.addStep(`^Validate FQDN fallback from \[(Secret|Environment)\] for PVC \[([^\]]+)\] expected \[([^\]]+)\] in namespace \[([^\]]+)\]$`, step.validateFQDNFallbackPV)
	runner.addStep(`^Validate TLS handshake timeout is set to \[(\d+)\] seconds for resource \[(\d+)\]$`, step.validateTLSHandshakeTimeout)
	runner.addStep(`^Validate PVC \[([^\]]+)\] is bound in namespace \[([^\]]+)\]$`, step.validatePVCBound)
	runner.addStep(`^Validate Pod \[([^\]]+)\] is running in namespace \[([^\]]+)\]$`, step.validatePodRunning)
	runner.addStep(`^Create PVC with non-mTLS StorageClass in namespace \[([^\]]+)\]$`, step.createPVCWithNonMTLSStorageClass)
	runner.addStep(`^Validate non-mTLS PVC is bound in namespace \[([^\]]+)\]$`, step.validateNonMTLSPVCBound)
	runner.addStep(`^Delete non-mTLS PVC in namespace \[([^\]]+)\]$`, step.deleteNonMTLSPVC)
	runner.addStep(`^Update Secret with template \[([^\]]+)\] name \[([^\]]+)\] in namespace \[([^\]]+)\] for \[([^\]]+)\]$`, step.updateSecretWithTemplate)
}

func (runner *Runner) addStep(expr string, stepFunc interface{}) {
	re := regexp.MustCompile(expr)

	v := reflect.ValueOf(stepFunc)
	typ := v.Type()
	if typ.Kind() != reflect.Func {
		panic(fmt.Sprintf("expected handler to be func, but got: %T", stepFunc))
	}

	if typ.NumOut() == 1 {
		typ = typ.Out(0)
		switch typ.Kind() {
		case reflect.Interface:
			if !typ.Implements(errorInterface) {
				panic(fmt.Sprintf("expected handler to return an error but got: %s", typ.Kind()))
			}
		default:
			panic(fmt.Sprintf("expected handler to return an error, but got: %s", typ.Kind()))
		}

	} else {
		panic(fmt.Sprintf("expected handler to return only one value, but got: %d", typ.NumOut()))
	}

	runner.Definitions = append(runner.Definitions, StepDefinition{
		Handler: v,
		Expr:    re,
	})
}

// RunStep - runs a step
func (runner *Runner) RunStep(stepName string, res Resource) error {
	// Support conditional execution: "If config.enableSftpSDC is true: ..."
	const conditionalPrefix = "If POWERFLEX_SDC_SFTP_REPO_ENABLED: "
	if len(stepName) > len(conditionalPrefix) && stepName[:len(conditionalPrefix)] == conditionalPrefix {
		if res.Scenario.Config["enableSftpSDC"] != "true" {
			// Skip the step if the config is not enabled
			fmt.Printf("             Skipping   %s\n", stepName[len(conditionalPrefix):])
			fmt.Println("             Reason: config.enableSftpSDC is not set to 'true'")

			return nil
		}
		// Run the actual step (remove the prefix)
		stepName = stepName[len(conditionalPrefix):]
	}

	for _, stepDef := range runner.Definitions {
		if stepDef.Expr.MatchString(stepName) {
			var values []reflect.Value
			groups := stepDef.Expr.FindStringSubmatch(stepName)

			typ := stepDef.Handler.Type()
			numArgs := typ.NumIn()
			if numArgs > len(groups) {
				return fmt.Errorf("expected handler method to take %d but got: %d", numArgs, len(groups))
			}

			values = append(values, reflect.ValueOf(res))
			for i := 1; i < len(groups); i++ {
				values = append(values, reflect.ValueOf(groups[i]))
			}

			res := stepDef.Handler.Call(values)
			if err, ok := res[0].Interface().(error); ok {
				fmt.Printf("             Retrying   %v\n", err)
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("no method for step: %s", stepName)
}

// StepTimeout returns an appropriate timeout for the given step name.
// Steps are categorized into fast (2 min), medium (10 min), and long (10 min).
func StepTimeout(stepName string) time.Duration {
	// Long-running steps (10 min): upgrades, third-party installs, custom tests, auth proxy config
	longPatterns := []string{
		"Upgrade from custom resource",
		"Install [",
		"Uninstall [",
		"Run custom test",
		"Run [",
		"Configure authorization-proxy-server",
		"Set operator failure grace period",
	}
	for _, p := range longPatterns {
		if strings.Contains(stepName, p) {
			return 10 * time.Minute
		}
	}

	// Medium steps (10 min): validation/installation checks that poll
	mediumPatterns := []string{
		"Validate",
	}
	for _, p := range mediumPatterns {
		if strings.Contains(stepName, p) {
			return 10 * time.Minute
		}
	}

	// Fast steps (2 min): apply, delete, create, set, enable/disable, etc.
	return 2 * time.Minute
}
