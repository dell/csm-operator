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

	CSIPowerMaxUseSecret       string = "X_CSI_REVPROXY_USE_SECRET"      // #nosec G101
	CSIPowerMaxSecretFilePath  string = "X_CSI_REVPROXY_SECRET_FILEPATH" // #nosec G101
	CSIPowerMaxSecretMountPath string = "/etc/powermax"                  // #nosec G101

	CSIPowerMaxSecretName       string = "config"                       // #nosec G101
	CSIPowerMaxSecretVolumeName string = "powermax-reverseproxy-secret" // #nosec G101

	CSIPowerMaxConfigPathKey   string = "X_CSI_POWERMAX_CONFIG_PATH"
	CSIPowerMaxConfigPathValue string = "/powermax-config-params/driver-config-params.yaml"
)

type CustomEnv struct {
	Name  string
	Value string
}

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

	useReverseProxySecret := UseReverseProxySecret(cr)
	if useReverseProxySecret {
		log.Infof("[PrecheckPowerMax] Using Secret: %s", secretName)
	} else {
		log.Infof("[PrecheckPowerMax] Using ConfigMap: %s", secretName)
	}

	found := &corev1.Secret{}
	err := ct.Get(ctx, types.NamespacedName{Name: secretName, Namespace: cr.GetNamespace()}, found)
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

	log.Debugw("preCheck", "secrets", secretName)
	return nil
}

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
			if *cnt.Name == "driver" || *cnt.Name == "reverseproxy" {
				setPowermaxMountCredentialContent(&podTemplate.Spec.Containers[i])
			}
		}

		return nil
	}

	for i, cnt := range podTemplate.Spec.Containers {
		if *cnt.Name == "driver" {
			SetPowermaxConfigContent(&podTemplate.Spec.Containers[i], secretName)
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
	contains := slices.ContainsFunc(ct.VolumeMounts,
		func(v acorev1.VolumeMountApplyConfiguration) bool { return *(v.Name) == *(mount.Name) },
	)

	if !contains {
		ct.VolumeMounts = append(ct.VolumeMounts, mount)
	}
}

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
	contains := slices.ContainsFunc(ct.Env,
		func(v acorev1.EnvVarApplyConfiguration) bool { return *(v.Name) == *(envVar.Name) },
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
