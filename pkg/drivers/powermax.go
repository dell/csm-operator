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
	"os"
	"strings"

	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/logger"
	"github.com/dell/csm-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Constant to be used for powermax deployment
const (
	// PowerMaxPluginIdentifier used to identify powermax plugin
	PowerMaxPluginIdentifier = "powermax"

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

	// CsiPmaxMaxVolumesPerNode - Maximum volumes that the controller can schedule on the node
	CsiPmaxMaxVolumesPerNode = "<X_CSI_MAX_VOLUMES_PER_NODE>"
)

// PrecheckPowerMax do input validation
func PrecheckPowerMax(ctx context.Context, cr *csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, ct client.Client) error {
	log := logger.GetLogger(ctx)
	// Check for default secret only
	// Array specific will be authenticated in csireverseproxy
	cred := cr.Name + "-creds"
	if cr.Spec.Driver.AuthSecret != "" {
		cred = cr.Spec.Driver.AuthSecret
	}

	// Check if driver version is supported by doing a stat on a config file
	configFilePath := fmt.Sprintf("%s/driverconfig/powermax/%s/upgrade-path.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Errorw("PreCheckPowerMax failed in version check", "Error", err.Error())
		return fmt.Errorf("%s %s not supported", csmv1.PowerMax, cr.Spec.Driver.ConfigVersion)
	}

	if cred != "" {
		found := &corev1.Secret{}
		err := ct.Get(ctx, types.NamespacedName{Name: cred, Namespace: cr.GetNamespace()}, found)
		if err != nil {
			log.Error(err, "Failed query for secret ", cred)
			if errors.IsNotFound(err) {
				return fmt.Errorf("failed to find secret %s", cred)
			}
		}
	}
	kubeletConfigDirFound := false
	for _, env := range cr.Spec.Driver.Common.Envs {
		if env.Name == "KUBELET_CONFIG_DIR" {
			kubeletConfigDirFound = true
		}
	}
	if !kubeletConfigDirFound {
		cr.Spec.Driver.Common.Envs = append(cr.Spec.Driver.Common.Envs, corev1.EnvVar{
			Name:  "KUBELET_CONFIG_DIR",
			Value: "/var/lib/kubelet",
		})
	}
	version, err := utils.GetLatestVersion(string(csmv1.ReverseProxy), operatorConfig)
	if err != nil {
		return err
	}
	if cr.Spec.Modules == nil {
		// This means it's a minimal yaml and we will append reverse-proxy by default
		modules := make([]csmv1.Module, 0)
		modules = append(modules, csmv1.Module{
			Name:              csmv1.ReverseProxy,
			Enabled:           true,
			ConfigVersion:     version,
			ForceRemoveModule: true,
			InitContainer:     nil,
		})
		cr.Spec.Modules = modules
	}

	foundRevProxy := false
	for _, mod := range cr.Spec.Modules {
		if mod.Name == csmv1.ReverseProxy {
			foundRevProxy = true
			break
		}
	}
	if !foundRevProxy {
		log.Infof("Reverse proxy module not found adding it with default config")
		cr.Spec.Modules = append(cr.Spec.Modules, csmv1.Module{
			Name:              csmv1.ReverseProxy,
			Enabled:           true,
			ConfigVersion:     version,
			ForceRemoveModule: true,
		})
	}

	log.Debugw("preCheck", "secrets", cred)
	return nil
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
	case "Controller":
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
		for _, env := range cr.Spec.Driver.Controller.Envs {
			if env.Name == "X_CSI_HEALTH_MONITOR_ENABLED" {
				ctrlHealthMonitor = env.Value
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
	case "CSIDriverSpec":
		if cr.Spec.Driver.CSIDriverSpec.StorageCapacity {
			storageCapacity = "true"
		}
		yamlString = strings.ReplaceAll(yamlString, CsiStorageCapacityEnabled, storageCapacity)
	}

	return yamlString
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
