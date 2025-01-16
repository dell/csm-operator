//  Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"strconv"
	"strings"

	v1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
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

	// CSIPmaxManagedArray and following  used for replacing user values in config files
	CSIPmaxManagedArray    = "<X_CSI_MANAGED_ARRAY>"
	CSIPmaxEndpoint        = "<X_CSI_POWERMAX_ENDPOINT>"
	CSIPmaxClusterPrefix   = "<X_CSI_K8S_CLUSTER_PREFIX>"
	CSIPmaxDebug           = "<X_CSI_POWERMAX_DEBUG>"
	CSIPmaxPortGroup       = "<X_CSI_POWERMAX_PORTGROUPS>"
	CSIPmaxProtocol        = "<X_CSI_TRANSPORT_PROTOCOL>"
	CSIPmaxNodeTemplate    = "<X_CSI_IG_NODENAME_TEMPLATE>"
	CSIPmaxModifyHostname  = "<X_CSI_IG_MODIFY_HOSTNAME>"
	CSIPmaxHealthMonitor   = "<X_CSI_HEALTH_MONITOR_ENABLED>"
	CSIPmaxTopology        = "<X_CSI_TOPOLOGY_CONTROL_ENABLED>"
	CSIPmaxVsphere         = "<X_CSI_VSPHERE_ENABLED>"
	CSIPmaxVspherePG       = "<X_CSI_VSPHERE_PORTGROUP>"
	CSIPmaxVsphereHostname = "<X_CSI_VSPHERE_HOSTNAME>"
	CSIPmaxVsphereHost     = "<X_CSI_VCENTER_HOST>"
	CSIPmaxChap            = "<X_CSI_POWERMAX_ISCSI_ENABLE_CHAP>"
	ReverseProxyTLSSecret  = "<X_CSI_REVPROXY_TLS_SECRET>" // #nosec G101

	// CsiPmaxMaxVolumesPerNode - Maximum volumes that the controller can schedule on the node
	CsiPmaxMaxVolumesPerNode = "<X_CSI_MAX_VOLUMES_PER_NODE>"

	// PowerMaxCSMNameSpace - namespace CSM is found in. Needed for cases where pod namespace is not namespace of CSM
	PowerMaxCSMNameSpace string = "<CSM_NAMESPACE>"

	CSIPowerMaxUseSecret       string = "X_CSI_REVPROXY_USE_SECRET"
	CSIPowerMaxSecretMountPath string = "/etc/powermax/"

	// To be used.
	CSIPowerMaxSecretName string = "powermax-config"
)

// PrecheckPowerMax do input validation
func PrecheckPowerMax(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, ct client.Client) error {
	log := logger.GetLogger(ctx)

	// Check if driver version is supported by doing a stat on a config file
	configFilePath := fmt.Sprintf("%s/driverconfig/powermax/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Errorw("PreCheckPowerMax failed in version check", "Error", err.Error())
		return fmt.Errorf("%s %s not supported", csmv1.PowerMax, cr.Spec.Driver.ConfigVersion)
	}

	secretName := cr.Name + "-creds"
	if cr.Spec.Driver.AuthSecret != "" {
		secretName = cr.Spec.Driver.AuthSecret
	}

	useReverseProxySecret := useReverseProxySecret(cr)
	if useReverseProxySecret {
		log.Infof("[FERNANDO] Using New Secret with name %s", secretName)
	} else {
		log.Infof("[FERNANDO] Using Old ConfigMap Secret with name %s", secretName)
	}

	found := &corev1.Secret{}
	err := ct.Get(ctx, types.NamespacedName{Name: secretName, Namespace: cr.GetNamespace()}, found)
	if err != nil {
		log.Error(err, "Failed query for secret ", secretName)
		if errors.IsNotFound(err) {
			return fmt.Errorf("failed to find secret %s", secretName)
		}
	} else {
		log.Infof("[FERNANDO] Secret %s found", secretName)
	}

	for i, mod := range cr.Spec.Modules {
		if mod.Name == csmv1.ReverseProxy {
			cr.Spec.Modules[i].Enabled = true
			cr.Spec.Modules[i].ForceRemoveModule = true
			break
		}
	}

	setUsageOfReverseProxySecret(cr, useReverseProxySecret)

	log.Debugw("preCheck", "secrets", secretName)
	return nil
}

func useReverseProxySecret(cr *csmv1.ContainerStorageModule) bool {
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

func setUsageOfReverseProxySecret(cr *csmv1.ContainerStorageModule, useSecret bool) {
	found := false

	var revProxy *csmv1.Module
	for _, mod := range cr.Spec.Modules {
		if mod.Name == csmv1.ReverseProxy {
			revProxy = &mod
		}
	}

	if revProxy == nil {
		log.Println("[FERNANDO] setUsageOfReverseProxySecret: could not find reverse proxy")
		return
	}

	for _, component := range revProxy.Components {
		if component.Name == ReverseProxyServerComponent {
			for i, env := range component.Envs {
				if env.Name == CSIPowerMaxUseSecret {
					revProxy.Components[0].Envs[i].Value = strconv.FormatBool(useSecret)
					found = true
				}
			}
		}
	}

	if !found {
		log.Println("[FERNANDO] setUsageOfReverseProxySecret: could not find", CSIPowerMaxUseSecret)
		revProxy.Components[0].Envs = append(revProxy.Components[0].Envs,
			corev1.EnvVar{Name: CSIPowerMaxUseSecret, Value: strconv.FormatBool(useSecret)},
		)
	}

	log.Printf("[FERNANDO] set usage of reverse proxy secret to %t", useSecret)
}

// ModifyPowermaxCR -
func ModifyPowermaxCR(yamlString string, cr csmv1.ContainerStorageModule, fileType string) string {
	// Parameters to initialise CR values
	managedArray := ""
	endpoint := ""
	clusterPrefix := ""
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
				if env.Name == "X_CSI_K8S_CLUSTER_PREFIX" {
					clusterPrefix = env.Value
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
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxClusterPrefix, clusterPrefix)
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
		yamlString = strings.ReplaceAll(yamlString, PowerMaxCSMNameSpace, cr.Namespace)
	case "Controller":
		if cr.Spec.Driver.Common != nil {
			for _, env := range cr.Spec.Driver.Common.Envs {
				if env.Name == "X_CSI_MANAGED_ARRAYS" {
					managedArray = env.Value
				}
				if env.Name == "X_CSI_POWERMAX_ENDPOINT" {
					endpoint = env.Value
				}
				if env.Name == "X_CSI_K8S_CLUSTER_PREFIX" {
					clusterPrefix = env.Value
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
			}
		}
		if cr.Spec.Driver.Controller != nil {
			for _, env := range cr.Spec.Driver.Controller.Envs {
				if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
					ctrlHealthMonitor = env.Value
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
		yamlString = strings.ReplaceAll(yamlString, CSIPmaxClusterPrefix, clusterPrefix)
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
		yamlString = strings.ReplaceAll(yamlString, PowerMaxCSMNameSpace, cr.Namespace)
	case "CSIDriverSpec":
		if cr.Spec.Driver.CSIDriverSpec != nil && cr.Spec.Driver.CSIDriverSpec.StorageCapacity {
			storageCapacity = "true"
		}
		yamlString = strings.ReplaceAll(yamlString, CsiStorageCapacityEnabled, storageCapacity)
	}

	return yamlString
}

func SetPowerMaxSecretMount(configuration interface{}, cr csmv1.ContainerStorageModule) (bool, error) {
	if useReverseProxySecret(&cr) {
		secretName := cr.Spec.Driver.AuthSecret
		optional := false
		mountPath := CSIPowerMaxSecretMountPath + secretName

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
			return false, fmt.Errorf("invalid type passed through")
		}

		// Adding volume
		podTemplate.Spec.Volumes = append(podTemplate.Spec.Volumes,
			acorev1.VolumeApplyConfiguration{Name: &secretName,
				VolumeSourceApplyConfiguration: acorev1.VolumeSourceApplyConfiguration{Secret: &acorev1.SecretVolumeSourceApplyConfiguration{SecretName: &secretName, Optional: &optional}}})

		// Adding volume mount for both the reverseproxy and driver
		for i, cnt := range podTemplate.Spec.Containers {
			if *cnt.Name == "driver" {
				podTemplate.Spec.Containers[i].VolumeMounts = append(podTemplate.Spec.Containers[i].VolumeMounts,
					acorev1.VolumeMountApplyConfiguration{Name: &secretName, MountPath: &mountPath})
			}
		}

		return true, nil
	}

	return false, nil
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
