//  Copyright © 2021 - 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package modules

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	certificate "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	csmv1 "github.com/dell/csm-operator/api/v1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/logger"
	utils "github.com/dell/csm-operator/pkg/utils"
	"golang.org/x/mod/semver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	// AuthDeploymentManifest - deployment resources and ingress rules for authorization module
	AuthDeploymentManifest = "deployment.yaml"
	// AuthIngressManifest -
	AuthIngressManifest = "ingress.yaml"
	// AuthCertManagerManifest -
	AuthCertManagerManifest = "cert-manager.yaml"
	// AuthNginxIngressManifest -
	AuthNginxIngressManifest = "nginx-ingress-controller.yaml"
	// AuthPolicyManifest -
	AuthPolicyManifest = "policies.yaml"
	// AuthLocalProvisionerManifest -
	AuthLocalProvisionerManifest = "local-provisioner.yaml"
	// AuthCustomCert - custom certificate file
	AuthCustomCert = "custom-cert.yaml"

	// AuthNamespace -
	AuthNamespace = "<NAMESPACE>"
	// AuthServerImage -
	AuthServerImage = "<AUTHORIZATION_PROXY_SERVER_IMAGE>"
	// AuthProxyServiceReplicas -
	AuthProxyServiceReplicas = "<AUTHORIZATION_PROXY_SERVICE_REPLICAS>"
	// AuthOpaImage -
	AuthOpaImage = "<AUTHORIZATION_OPA_IMAGE>"
	// AuthOpaKubeMgmtImage -
	AuthOpaKubeMgmtImage = "<AUTHORIZATION_OPA_KUBEMGMT_IMAGE>"
	// AuthTenantServiceImage -
	AuthTenantServiceImage = "<AUTHORIZATION_TENANT_SERVICE_IMAGE>"
	// AuthTenantServiceReplicas -
	AuthTenantServiceReplicas = "<AUTHORIZATION_TENANT_SERVICE_REPLICAS>"
	// AuthRoleServiceImage -
	AuthRoleServiceImage = "<AUTHORIZATION_ROLE_SERVICE_IMAGE>"
	// AuthRoleServiceReplicas -
	AuthRoleServiceReplicas = "<AUTHORIZATION_ROLE_SERVICE_REPLICAS>"
	// AuthStorageServiceImage -
	AuthStorageServiceImage = "<AUTHORIZATION_STORAGE_SERVICE_IMAGE>"
	// AuthStorageServiceReplicas -
	AuthStorageServiceReplicas = "<AUTHORIZATION_STORAGE_SERVICE_REPLICAS>"
	// AuthRedisImage -
	AuthRedisImage = "<AUTHORIZATION_REDIS_IMAGE>"
	// AuthRedisCommanderImage -
	AuthRedisCommanderImage = "<AUTHORIZATION_REDIS_COMMANDER_IMAGE>"
	// AuthRedisStorageClass -
	AuthRedisStorageClass = "<REDIS_STORAGE_CLASS>"
	// AuthControllerImage -
	AuthControllerImage = "<AUTHORIZATION_CONTROLLER_IMAGE>"
	// AuthControllerReplicas -
	AuthControllerReplicas = "<AUTHORIZATION_CONTROLLER_REPLICAS>"
	// AuthLeaderElectionEnabled -
	AuthLeaderElectionEnabled = "<AUTHORIZATION_LEADER_ELECTION_ENABLED>"
	// AuthControllerReconcileInterval -
	AuthControllerReconcileInterval = "<AUTHORIZATION_CONTROLLER_RECONCILE_INTERVAL>"

	// AuthProxyHost -
	AuthProxyHost = "<AUTHORIZATION_HOSTNAME>"
	// AuthProxyIngressHost -
	AuthProxyIngressHost = "<PROXY_INGRESS_HOST>"

	// AuthVaultAddress -
	AuthVaultAddress = "<AUTHORIZATION_VAULT_ADDRESS>"
	// AuthVaultRole -
	AuthVaultRole = "<AUTHORIZATION_VAULT_ROLE>"
	// AuthSkipCertificateValidation -
	AuthSkipCertificateValidation = "<AUTHORIZATION_SKIP_CERTIFICATE_VALIDATION>"
	// AuthKvEnginePath -
	AuthKvEnginePath = "<AUTHORIZATION_KV_ENGINE_PATH>"
	// AuthRedisName -
	AuthRedisName = "<AUTHORIZATION_REDIS_NAME>"
	// AuthRedisCommander -
	AuthRedisCommander = "<AUTHORIZATION_REDIS_COMMANDER>"
	// AuthRedisSentinel -
	AuthRedisSentinel = "<AUTHORIZATION_REDIS_SENTINEL>"
	// AuthRedisSentinelValues -
	AuthRedisSentinelValues = "<AUTHORIZATION_REDIS_SENTINEL_VALUES>"
	// AuthRedisReplicas -
	AuthRedisReplicas = "<AUTHORIZATION_REDIS_REPLICAS>"

	// AuthCert - for tls secret
	AuthCert = "<BASE64_CERTIFICATE>"
	// AuthPrivateKey - for tls secret
	AuthPrivateKey = "<BASE64_PRIVATE_KEY>"

	// AuthProxyServerComponent - proxy-server component
	AuthProxyServerComponent = "proxy-server"
	// AuthSidecarComponent - karavi-authorization-proxy component
	AuthSidecarComponent = "karavi-authorization-proxy"
	// AuthNginxIngressComponent - nginx component
	AuthNginxIngressComponent = "nginx"
	// AuthCertManagerComponent - cert-manager component
	AuthCertManagerComponent = "cert-manager"
	// AuthRedisComponent - redis component
	AuthRedisComponent = "redis"
	// AuthVaultComponent - vault component
	AuthVaultComponent = "vault"

	// AuthLocalStorageClass -
	AuthLocalStorageClass = "csm-authorization-local-storage"

	// AuthCrds - name of authorization crd manifest yaml
	AuthCrds = "authorization-crds.yaml"
)

var (
	redisStorageClass     string
	authHostname          string
	proxyIngressClassName string
	authCertificate       string
	authPrivateKey        string
	secretName            string

	pathType    = networking.PathTypePrefix
	duration    = 2160 * time.Hour // 90d
	renewBefore = 360 * time.Hour  // 15d
)

// AuthorizationSupportedDrivers ... is a map containing the CSI Drivers supported by CSM Authorization. The key is driver name and the value is the driver plugin identifier
var AuthorizationSupportedDrivers = map[string]SupportedDriverParam{
	"powerscale": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	},
	"isilon": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	},
	"powerflex": {
		PluginIdentifier:              drivers.PowerFlexPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerFlexConfigParamsVolumeMount,
	},
	"vxflexos": {
		PluginIdentifier:              drivers.PowerFlexPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerFlexConfigParamsVolumeMount,
	}, // powerscale/isilon & powerflex/vxflexos are valid types
	"powermax": {
		PluginIdentifier:              drivers.PowerMaxPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerMaxConfigParamsVolumeMount,
	},
}

func getAuthorizationModule(cr csmv1.ContainerStorageModule) (csmv1.Module, error) {
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.AuthorizationServer {
			return m, nil
		}
	}
	return csmv1.Module{}, fmt.Errorf("authorization module not found")
}

// CheckAnnotationAuth --
func CheckAnnotationAuth(annotation map[string]string) error {
	if annotation != nil {
		fmt.Println(annotation)
		if _, ok := annotation["com.dell.karavi-authorization-proxy"]; !ok {
			return errors.New("com.dell.karavi-authorization-proxy is missing from annotation")
		}
		if annotation["com.dell.karavi-authorization-proxy"] != "true" {
			return fmt.Errorf("expected notation value to be true but got %s", annotation["com.dell.karavi-authorization-proxy"])
		}
		return nil
	}
	return errors.New("annotation is nil")
}

// CheckApplyVolumesAuth --
func CheckApplyVolumesAuth(volumes []acorev1.VolumeApplyConfiguration) error {
	// Volume
	volumeNames := []string{"karavi-authorization-config"}
NAME_LOOP:
	for _, volName := range volumeNames {
		for _, vol := range volumes {
			if *vol.Name == volName {
				continue NAME_LOOP
			}
		}
		return fmt.Errorf("missing the following volume %s", volName)
	}

	return nil
}

// CheckApplyContainersAuth --
func CheckApplyContainersAuth(containers []acorev1.ContainerApplyConfiguration, drivertype string, skipCertificateValidation bool) error {
	authString := "karavi-authorization-proxy"
	for _, cnt := range containers {
		if *cnt.Name == authString {
			volumeMounts := []string{"karavi-authorization-config", AuthorizationSupportedDrivers[drivertype].DriverConfigParamsVolumeMount}
		MOUNT_NAME_LOOP:
			for _, volName := range volumeMounts {
				for _, vol := range cnt.VolumeMounts {
					if *vol.Name == volName {
						continue MOUNT_NAME_LOOP
					}
				}
				return fmt.Errorf("missing the following volume mount %s", volName)
			}

			for _, env := range cnt.Env {
				if *env.Name == "SKIP_CERTIFICATE_VALIDATION" || *env.Name == "INSECURE" {
					if _, err := strconv.ParseBool(*env.Value); err != nil {
						return fmt.Errorf("%s is an invalid value for SKIP_CERTIFICATE_VALIDATION: %v", *env.Value, err)
					}

					if skipCertificateValidation {
						if *env.Value != "true" {
							return fmt.Errorf("expected SKIP_CERTIFICATE_VALIDATION/INSECURE to be true")
						}
					} else {
						if *env.Value != "false" {
							return fmt.Errorf("expected SKIP_CERTIFICATE_VALIDATION/INSECURE to be false")
						}
					}
				}
				if *env.Name == "PROXY_HOST" && *env.Value == "" {
					return fmt.Errorf("PROXY_HOST for authorization is empty")
				}
			}
			return nil
		}
	}
	return errors.New("karavi-authorization-proxy container was not injected into driver")
}

func getAuthApplyCR(cr csmv1.ContainerStorageModule, op utils.OperatorConfig) (*csmv1.Module, *acorev1.ContainerApplyConfiguration, error) {
	var err error
	authModule := csmv1.Module{}
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Authorization {
			authModule = m
			break
		}
	}

	authConfigVersion := authModule.ConfigVersion
	if authConfigVersion == "" {
		authConfigVersion, err = utils.GetModuleDefaultVersion(cr.Spec.Driver.ConfigVersion, cr.Spec.Driver.CSIDriverType, csmv1.Authorization, op.ConfigDirectory)
		if err != nil {
			return nil, nil, err
		}
	}

	configMapPath := fmt.Sprintf("%s/moduleconfig/authorization/%s/container.yaml", op.ConfigDirectory, authConfigVersion)
	buf, err := os.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		return nil, nil, err
	}

	YamlString := utils.ModifyCommonCR(string(buf), cr)

	YamlString = strings.ReplaceAll(YamlString, DefaultPluginIdentifier, AuthorizationSupportedDrivers[string(cr.Spec.Driver.CSIDriverType)].PluginIdentifier)

	var container acorev1.ContainerApplyConfiguration
	err = yaml.Unmarshal([]byte(YamlString), &container)
	if err != nil {
		return nil, nil, err
	}

	for i, component := range authModule.Components {
		if component.Name == "karavi-authorization-proxy" {
			skipcertFound := false
			for _, env := range authModule.Components[i].Envs {
				if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
					skipcertFound = true
					break
				}
			}
			// If SKIP_CERTIFICATE_VALIDATION is not found, add it
			if !skipcertFound {
				authModule.Components[i].Envs = append(authModule.Components[i].Envs, corev1.EnvVar{
					Name:  "SKIP_CERTIFICATE_VALIDATION",
					Value: "true",
				})
			}
		}
	}
	container.Env = utils.ReplaceAllApplyCustomEnvs(container.Env, authModule.Components[0].Envs, authModule.Components[0].Envs)

	skipCertValid := false
	for _, env := range authModule.Components[0].Envs {
		if env.Name == "INSECURE" || env.Name == "SKIP_CERTIFICATE_VALIDATION" {
			skipCertValid, _ = strconv.ParseBool(env.Value)
		}
	}

	certString := "proxy-server-root-certificate"
	if skipCertValid { // do not mount proxy-server-root-certificate
		for i, c := range container.VolumeMounts {
			if *c.Name == certString {
				container.VolumeMounts[i] = container.VolumeMounts[len(container.VolumeMounts)-1]
				container.VolumeMounts = container.VolumeMounts[:len(container.VolumeMounts)-1]
			}
		}
	} else {
		for i, e := range container.Env {
			if *e.Name == "INSECURE" || *e.Name == "SKIP_CERTIFICATE_VALIDATION" {
				value := strconv.FormatBool(skipCertValid)
				container.Env[i].Value = &value
			}
		}
	}
	for i, c := range container.VolumeMounts {
		if *c.Name == DefaultDriverConfigParamsVolumeMount {
			newName := AuthorizationSupportedDrivers[string(cr.Spec.Driver.CSIDriverType)].DriverConfigParamsVolumeMount
			container.VolumeMounts[i].Name = &newName
			break
		}
	}

	return &authModule, &container, nil
}

func getAuthApplyVolumes(cr csmv1.ContainerStorageModule, op utils.OperatorConfig, auth csmv1.ContainerTemplate) ([]acorev1.VolumeApplyConfiguration, error) {
	version, err := utils.GetModuleDefaultVersion(cr.Spec.Driver.ConfigVersion, cr.Spec.Driver.CSIDriverType, csmv1.Authorization, op.ConfigDirectory)
	if err != nil {
		return nil, err
	}

	configMapPath := fmt.Sprintf("%s/moduleconfig/authorization/%s/volumes.yaml", op.ConfigDirectory, version)
	buf, err := os.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		return nil, err
	}

	var vols []acorev1.VolumeApplyConfiguration
	err = yaml.Unmarshal(buf, &vols)
	if err != nil {
		return nil, err
	}

	skipCertValid := false
	for _, env := range auth.Envs {
		if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
			skipCertValid, _ = strconv.ParseBool(env.Value)
		}
	}

	certString := "proxy-server-root-certificate"
	if skipCertValid { // do not mount proxy-server-root-certificate
		for i, c := range vols {
			if *c.Name == certString {
				vols[i] = vols[len(vols)-1]
				return vols[:len(vols)-1], nil

			}
		}
	}
	return vols, nil
}

// AuthInjectDaemonset  - inject authorization into daemonset
func AuthInjectDaemonset(ds applyv1.DaemonSetApplyConfiguration, cr csmv1.ContainerStorageModule, op utils.OperatorConfig) (*applyv1.DaemonSetApplyConfiguration, error) {
	authModule, containerPtr, err := getAuthApplyCR(cr, op)
	if err != nil {
		return nil, err
	}

	container := *containerPtr
	utils.UpdateSideCarApply(authModule.Components, &container)

	vols, err := getAuthApplyVolumes(cr, op, authModule.Components[0])
	if err != nil {
		return nil, err
	}

	if ds.Annotations != nil {
		ds.Annotations["com.dell.karavi-authorization-proxy"] = "true"
	} else {
		ds.Annotations = map[string]string{
			"com.dell.karavi-authorization-proxy": "true",
		}
	}
	ds.Spec.Template.Spec.Containers = append(ds.Spec.Template.Spec.Containers, container)
	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, vols...)

	return &ds, nil
}

// AuthInjectDeployment - inject authorization into deployment
func AuthInjectDeployment(dp applyv1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule, op utils.OperatorConfig) (*applyv1.DeploymentApplyConfiguration, error) {
	authModule, containerPtr, err := getAuthApplyCR(cr, op)
	if err != nil {
		return nil, err
	}

	container := *containerPtr
	utils.UpdateSideCarApply(authModule.Components, &container)

	vols, err := getAuthApplyVolumes(cr, op, authModule.Components[0])
	if err != nil {
		return nil, err
	}

	if dp.Annotations != nil {
		dp.Annotations["com.dell.karavi-authorization-proxy"] = "true"
	} else {
		dp.Annotations = map[string]string{
			"com.dell.karavi-authorization-proxy": "true",
		}
	}
	dp.Spec.Template.Spec.Containers = append(dp.Spec.Template.Spec.Containers, container)
	dp.Spec.Template.Spec.Volumes = append(dp.Spec.Template.Spec.Volumes, vols...)

	return &dp, nil
}

// AuthorizationPrecheck  - runs precheck for CSM Authorization
func AuthorizationPrecheck(ctx context.Context, op utils.OperatorConfig, auth csmv1.Module, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	log := logger.GetLogger(ctx)
	if _, ok := AuthorizationSupportedDrivers[string(cr.Spec.Driver.CSIDriverType)]; !ok {
		return fmt.Errorf("CSM Authorization does not support %s driver", string(cr.Spec.Driver.CSIDriverType))
	}

	// check if provided version is supported
	if auth.ConfigVersion != "" {
		err := checkVersion(string(csmv1.Authorization), auth.ConfigVersion, op.ConfigDirectory)
		if err != nil {
			return err
		}
	}

	// Check for secrets
	skipCertValid := false
	// check if components are present or not
	for i, component := range auth.Components {
		if component.Name == "karavi-authorization-proxy" {
			skipcertFound := false
			for _, env := range auth.Components[i].Envs {
				if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
					skipcertFound = true
					break
				}
			}
			// If SKIP_CERTIFICATE_VALIDATION is not found, add it
			if !skipcertFound {
				auth.Components[i].Envs = append(auth.Components[i].Envs, corev1.EnvVar{
					Name:  "SKIP_CERTIFICATE_VALIDATION",
					Value: "true",
				})
			}
		}
	}

	for _, env := range auth.Components[0].Envs {
		if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
			b, err := strconv.ParseBool(env.Value)
			if err != nil {
				return fmt.Errorf("%s is an invalid value for SKIP_CERTIFICATE_VALIDATION: %v", env.Value, err)
			}
			skipCertValid = b
		}
		if env.Name == "PROXY_HOST" && env.Value == "" {
			return fmt.Errorf("PROXY_HOST for authorization is empty")
		}
	}

	secrets := []string{"karavi-authorization-config", "proxy-authz-tokens"}
	if !skipCertValid {
		secrets = append(secrets, "proxy-server-root-certificate")
	}

	for _, name := range secrets {
		found := &corev1.Secret{}
		err := ctrlClient.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: cr.GetNamespace(),
		}, found)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return fmt.Errorf("failed to find secret %s and certificate validation is requested", name)
			}
			log.Error(err, "Failed to query for secret. Warning - the controller pod may not start")
		}
	}

	log.Infof("preformed pre-checks for %s", auth.Name)
	return nil
}

// AuthorizationServerPrecheck  - runs precheck for CSM Authorization Proxy Server
func AuthorizationServerPrecheck(ctx context.Context, op utils.OperatorConfig, auth csmv1.Module, cr csmv1.ContainerStorageModule, r utils.ReconcileCSM) error {
	log := logger.GetLogger(ctx)

	if auth.ConfigVersion != "" {
		err := checkVersion(string(csmv1.Authorization), auth.ConfigVersion, op.ConfigDirectory)
		if err != nil {
			return err
		}
	}

	// Check for secrets
	var proxyServerSecrets []string
	switch semver.Major(auth.ConfigVersion) {
	case "v2":
		proxyServerSecrets = []string{"karavi-config-secret"}
	case "v1":
		proxyServerSecrets = []string{"karavi-config-secret", "karavi-storage-secret"}
	default:
		return fmt.Errorf("authorization major version %s not supported", semver.Major(auth.ConfigVersion))
	}
	for _, name := range proxyServerSecrets {
		found := &corev1.Secret{}
		err := r.GetClient().Get(ctx, types.NamespacedName{Name: name, Namespace: cr.GetNamespace()}, found)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return fmt.Errorf("failed to find secret %s", name)
			}
		}
	}

	log.Infof("preformed pre-checks for %s proxy server", auth.Name)
	return nil
}

// getAuthorizationServerDeployment - apply dynamic values to the deployment manifest before installation
func getAuthorizationServerDeployment(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""
	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return YamlString, err
	}

	buf, err := readConfigFile(auth, cr, op, AuthDeploymentManifest)
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	authNamespace := cr.Namespace

	for _, component := range auth.Components {
		// proxy-server component
		if component.Name == AuthProxyServerComponent {
			YamlString = strings.ReplaceAll(YamlString, AuthServerImage, component.ProxyService)
			YamlString = strings.ReplaceAll(YamlString, AuthProxyServiceReplicas, strconv.Itoa(component.ProxyServiceReplicas))
			YamlString = strings.ReplaceAll(YamlString, AuthOpaImage, component.Opa)
			YamlString = strings.ReplaceAll(YamlString, AuthOpaKubeMgmtImage, component.OpaKubeMgmt)
			YamlString = strings.ReplaceAll(YamlString, AuthTenantServiceImage, component.TenantService)
			YamlString = strings.ReplaceAll(YamlString, AuthTenantServiceReplicas, strconv.Itoa(component.TenantServiceReplicas))
			YamlString = strings.ReplaceAll(YamlString, AuthRoleServiceImage, component.RoleService)
			YamlString = strings.ReplaceAll(YamlString, AuthRoleServiceReplicas, strconv.Itoa(component.RoleServiceReplicas))
			YamlString = strings.ReplaceAll(YamlString, AuthStorageServiceImage, component.StorageService)
			YamlString = strings.ReplaceAll(YamlString, AuthStorageServiceReplicas, strconv.Itoa(component.StorageServiceReplicas))
			YamlString = strings.ReplaceAll(YamlString, AuthControllerImage, component.AuthorizationController)
			YamlString = strings.ReplaceAll(YamlString, AuthControllerReplicas, strconv.Itoa(component.AuthorizationControllerReplicas))
			YamlString = strings.ReplaceAll(YamlString, AuthLeaderElectionEnabled, strconv.FormatBool(component.LeaderElection))
			YamlString = strings.ReplaceAll(YamlString, AuthControllerReconcileInterval, component.ControllerReconcileInterval)
			YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)
		}

		// redis component
		if component.Name == AuthRedisComponent {
			YamlString = strings.ReplaceAll(YamlString, AuthRedisImage, component.Redis)
			YamlString = strings.ReplaceAll(YamlString, AuthRedisCommanderImage, component.Commander)
			YamlString = strings.ReplaceAll(YamlString, AuthRedisName, component.RedisName)
			YamlString = strings.ReplaceAll(YamlString, AuthRedisCommander, component.RedisCommander)
			YamlString = strings.ReplaceAll(YamlString, AuthRedisSentinel, component.Sentinel)
			YamlString = strings.ReplaceAll(YamlString, AuthRedisReplicas, strconv.Itoa(component.RedisReplicas))

			var sentinelValues []string
			for i := 0; i < component.RedisReplicas; i++ {
				sentinelValues = append(sentinelValues, fmt.Sprintf("sentinel-%d.sentinel.%s.svc.cluster.local:5000", i, authNamespace))
			}
			sentinels := strings.Join(sentinelValues, ", ")
			YamlString = strings.ReplaceAll(YamlString, AuthRedisSentinelValues, sentinels)

			if component.RedisStorageClass == "" {
				redisStorageClass = AuthLocalStorageClass
			} else {
				redisStorageClass = component.RedisStorageClass
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, AuthNamespace, authNamespace)
	YamlString = strings.ReplaceAll(YamlString, AuthRedisStorageClass, redisStorageClass)
	YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)

	return YamlString, nil
}

// getAuthorizationLocalProvisioner for redis
func getAuthorizationLocalProvisioner(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (bool, string, error) {
	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return false, "", err
	}

	for _, component := range auth.Components {
		if component.Name == AuthRedisComponent {
			if component.RedisStorageClass == "" {
				buf, err := readConfigFile(auth, cr, op, AuthLocalProvisionerManifest)
				if err != nil {
					return false, "", err
				}
				return true, string(buf), nil
			}
		}
	}
	return false, "", nil
}

// AuthorizationServerDeployment - apply/delete deployment objects
func AuthorizationServerDeployment(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	useLocalStorage, yamlString, err := getAuthorizationLocalProvisioner(op, cr)
	if err != nil {
		return err
	}

	if useLocalStorage {
		err = applyDeleteObjects(ctx, ctrlClient, yamlString, isDeleting)
		if err != nil {
			return err
		}
	}

	YamlString, err := getAuthorizationServerDeployment(op, cr)
	if err != nil {
		return err
	}

	err = applyDeleteObjects(ctx, ctrlClient, YamlString, isDeleting)
	if err != nil {
		return err
	}

	err = applyDeleteAuthorizationStorageService(ctx, isDeleting, cr, ctrlClient)
	if err != nil {
		return err
	}

	return nil
}

// AuthorizationStorageService - apply/delete storage service deployment and volume objects
func applyDeleteAuthorizationStorageService(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	switch semver.Major(authModule.ConfigVersion) {
	case "v2":
		return authorizationStorageServiceV2(ctx, isDeleting, cr, ctrlClient)
	case "v1":
		return authorizationStorageServiceV1(ctx, isDeleting, cr, ctrlClient)
	default:
		return fmt.Errorf("authorization major version %s not supported", semver.Major(authModule.ConfigVersion))
	}
}

func authorizationStorageServiceV1(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	// get component variables
	image := ""
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthProxyServerComponent:
			image = component.StorageService
		}
	}

	deployment := getStorageServiceScaffold(cr.Name, cr.Namespace, image, 1)

	// set karavi-storage-secret volume
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "storage-volume",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "karavi-storage-secret",
			},
		},
	})
	for i, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "storage-service" {
			deployment.Spec.Template.Spec.Containers[i].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
				Name:      "storage-volume",
				MountPath: "/etc/karavi-authorization/storage",
			})
			break
		}
	}

	deploymentBytes, err := json.Marshal(&deployment)
	if err != nil {
		return fmt.Errorf("marshalling storage-service deployment: %w", err)
	}

	deploymentYaml, err := yaml.JSONToYAML(deploymentBytes)
	if err != nil {
		return fmt.Errorf("converting storage-service json to yaml: %w", err)
	}

	return applyDeleteObjects(ctx, ctrlClient, string(deploymentYaml), isDeleting)
}

func authorizationStorageServiceV2(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	err = applyDeleteVaultCertificates(ctx, isDeleting, cr, ctrlClient)
	if err != nil {
		return fmt.Errorf("applying/deleting vault certificates: %w", err)
	}

	replicas := 0
	sentinels := ""
	image := ""
	vaults := []csmv1.Vault{}
	leaderElection := true
	otelCollector := ""
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthProxyServerComponent:
			replicas = component.StorageServiceReplicas
			image = component.StorageService
			leaderElection = component.LeaderElection
			otelCollector = component.OpenTelemetryCollectorAddress
		case AuthRedisComponent:
			var sentinelValues []string
			for i := 0; i < component.RedisReplicas; i++ {
				sentinelValues = append(sentinelValues, fmt.Sprintf("sentinel-%d.sentinel.%s.svc.cluster.local:5000", i, cr.Namespace))
			}
			sentinels = strings.Join(sentinelValues, ", ")
		case AuthVaultComponent:
			vaults = component.Vaults
		default:
			continue
		}
	}

	// conversion to int32 is safe for a value up to 2147483647
	// #nosec G115
	deployment := getStorageServiceScaffold(cr.Name, cr.Namespace, image, int32(replicas))

	// set vault volumes

	for _, vault := range vaults {
		volume := corev1.Volume{
			Name: fmt.Sprintf("vault-client-certificate-%s", vault.Identifier),
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{}},
				},
			},
		}

		if vault.CertificateAuthority != "" {
			volume.VolumeSource.Projected.Sources = append(volume.VolumeSource.Projected.Sources, corev1.VolumeProjection{
				Secret: &corev1.SecretProjection{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("vault-certificate-authority-%s", vault.Identifier),
					},
				},
			})
		}

		if vault.ClientCertificate != "" && vault.ClientKey != "" {
			volume.VolumeSource.Projected.Sources = append(volume.VolumeSource.Projected.Sources, corev1.VolumeProjection{
				Secret: &corev1.SecretProjection{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("vault-client-certificate-%s", vault.Identifier),
					},
				},
			})
		} else {
			volume.VolumeSource.Projected.Sources = append(volume.VolumeSource.Projected.Sources, corev1.VolumeProjection{
				Secret: &corev1.SecretProjection{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("storage-service-selfsigned-tls-%s", vault.Identifier),
					},
				},
			})
		}

		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, volume)
	}

	for i, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "storage-service" {
			for _, vault := range vaults {
				deployment.Spec.Template.Spec.Containers[i].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
					Name:      fmt.Sprintf("vault-client-certificate-%s", vault.Identifier),
					MountPath: fmt.Sprintf("/etc/vault/%s", vault.Identifier),
				})
			}
			break
		}
	}

	// set redis envs
	redis := []corev1.EnvVar{
		{
			Name:  "SENTINELS",
			Value: sentinels,
		},
		{
			Name: "REDIS_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "redis-csm-secret",
					},
					Key: "password",
				},
			},
		},
	}
	for i, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "storage-service" {
			deployment.Spec.Template.Spec.Containers[i].Env = append(deployment.Spec.Template.Spec.Containers[i].Env, redis...)
			break
		}
	}

	// set arguments
	var vaultArgs []string
	for _, vault := range vaults {
		vaultArgs = append(vaultArgs, fmt.Sprintf("--vault=%s,%s,%s,%t", vault.Identifier, vault.Address, vault.Role, vault.SkipCertificateValidation))
	}

	args := []string{
		"--redis-sentinel=$(SENTINELS)",
		"--redis-password=$(REDIS_PASSWORD)",
		fmt.Sprintf("--leader-election=%t", leaderElection),
	}

	// if the config version is greater than v2.0.0-alpha, add the collector-address arg
	if semver.Compare(authModule.ConfigVersion, "v2.0.0-alpha") == 1 {
		args = append(args, fmt.Sprintf("--collector-address=%s", otelCollector))
	}
	args = append(args, vaultArgs...)

	for i, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "storage-service" {
			deployment.Spec.Template.Spec.Containers[i].Args = append(deployment.Spec.Template.Spec.Containers[i].Args, args...)
			break
		}
	}

	// if the config version is greater than v2.0.0-alpha, set promhttp container port
	if semver.Compare(authModule.ConfigVersion, "v2.0.0-alpha") == 1 {
		for i, c := range deployment.Spec.Template.Spec.Containers {
			if c.Name == "storage-service" {
				deployment.Spec.Template.Spec.Containers[i].Ports = append(deployment.Spec.Template.Spec.Containers[i].Ports,
					corev1.ContainerPort{
						Name:          "promhttp",
						Protocol:      "TCP",
						ContainerPort: 2112,
					},
				)
				break
			}
		}
	}

	deploymentBytes, err := json.Marshal(&deployment)
	if err != nil {
		return fmt.Errorf("marshalling storage-service deployment: %w", err)
	}

	deploymentYaml, err := yaml.JSONToYAML(deploymentBytes)
	if err != nil {
		return fmt.Errorf("converting storage-service json to yaml: %w", err)
	}

	err = applyDeleteObjects(ctx, ctrlClient, string(deploymentYaml), isDeleting)
	if err != nil {
		return fmt.Errorf("applying storage-service deployment: %w", err)
	}
	return nil
}

// getStorageServiceScaffold returns the storage-service deployment with the common elements between v1 and v2
// callers must ensure that other elements specific for the version get set in the returned deployment
func getStorageServiceScaffold(name string, namespace string, image string, replicas int32) appsv1.Deployment {
	return appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storage-service",
			Namespace: namespace,
			Labels: map[string]string{
				"app": "storage-service",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "storage-service",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"csm": name,
						"app": "storage-service",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "storage-service",
					Containers: []corev1.Container{
						{
							Name:            "storage-service",
							Image:           image,
							ImagePullPolicy: "Always",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 50051,
									Name:          "grpc",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "NAMESPACE",
									Value: namespace,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-volume",
									MountPath: "/etc/karavi-authorization/config",
								},
								{
									Name:      "csm-config-params",
									MountPath: "/etc/karavi-authorization/csm-config-params",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "karavi-config-secret",
								},
							},
						},
						{
							Name: "csm-config-params",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "csm-config-params",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func applyDeleteVaultCertificates(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	// get vault certificate data from CR
	vaults := []csmv1.Vault{}
loop:
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthVaultComponent:
			vaults = component.Vaults
			break loop
		default:
			continue
		}
	}

	for _, vault := range vaults {
		if vault.CertificateAuthority != "" {
			vaultCABytes, err := base64.StdEncoding.DecodeString(vault.CertificateAuthority)
			if err != nil {
				return fmt.Errorf("decoding vault certificate authority: %w", err)
			}

			secret := corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("vault-certificate-authority-%s", vault.Identifier),
					Namespace: cr.Namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"ca.crt": vaultCABytes,
				},
			}

			secretBytes, err := json.Marshal(&secret)
			if err != nil {
				return fmt.Errorf("marshalling vault certificate authority secret: %w", err)
			}

			yamlString, err := yaml.JSONToYAML(secretBytes)
			if err != nil {
				return fmt.Errorf("converting vault certificate authority json to yaml: %w", err)
			}

			err = applyDeleteObjects(ctx, ctrlClient, string(yamlString), isDeleting)
			if err != nil {
				return fmt.Errorf("applying vault certificate authority secret: %w", err)
			}
		}

		if vault.ClientCertificate != "" && vault.ClientKey != "" {
			vaultCertBytes, err := base64.StdEncoding.DecodeString(vault.ClientCertificate)
			if err != nil {
				return fmt.Errorf("decoding vault certificate: %w", err)
			}

			vaultKeyBytes, err := base64.StdEncoding.DecodeString(vault.ClientKey)
			if err != nil {
				return fmt.Errorf("decoding vault private key: %w", err)
			}

			secret := corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("vault-client-certificate-%s", vault.Identifier),
					Namespace: cr.Namespace,
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					"tls.crt": vaultCertBytes,
					"tls.key": vaultKeyBytes,
				},
			}

			secretBytes, err := json.Marshal(&secret)
			if err != nil {
				return fmt.Errorf("marshalling vault certificate secret: %w", err)
			}

			yamlString, err := yaml.JSONToYAML(secretBytes)
			if err != nil {
				return fmt.Errorf("converting vault certificate json to yaml: %w", err)
			}

			err = applyDeleteObjects(ctx, ctrlClient, string(yamlString), isDeleting)
			if err != nil {
				return fmt.Errorf("applying vault certificate secret: %w", err)
			}
		} else {
			issuer := createSelfSignedIssuer(cr, fmt.Sprintf("storage-service-selfsigned-%s", vault.Identifier))

			issuerByes, err := json.Marshal(issuer)
			if err != nil {
				return fmt.Errorf("marshaling storage-service-selfsigned issuer: %v", err)
			}

			issuerYaml, err := yaml.JSONToYAML(issuerByes)
			if err != nil {
				return fmt.Errorf("converting storage-service-selfsigned issuer json to yaml: %v", err)
			}

			// create/delete issuer
			err = applyDeleteObjects(ctx, ctrlClient, string(issuerYaml), isDeleting)
			if err != nil {
				return err
			}

			certificate := createSelfSignedCertificate(
				cr,
				[]string{fmt.Sprintf("storage-service.%s.svc.cluster.local", cr.Namespace)},
				fmt.Sprintf("storage-service-selfsigned-%s", vault.Identifier),
				fmt.Sprintf("storage-service-selfsigned-tls-%s", vault.Identifier),
				fmt.Sprintf("storage-service-selfsigned-%s", vault.Identifier))

			certBytes, err := json.Marshal(certificate)
			if err != nil {
				return fmt.Errorf("marshaling storage-service-selfsigned certificate: %v", err)
			}

			certYaml, err := yaml.JSONToYAML(certBytes)
			if err != nil {
				return fmt.Errorf("converting storage-service-selfsigned certificate json to yaml: %v", err)
			}

			// create/delete certificate
			err = applyDeleteObjects(ctx, ctrlClient, string(certYaml), isDeleting)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// AuthorizationIngress - apply/delete ingress objects
func AuthorizationIngress(ctx context.Context, isDeleting, isOpenShift bool, cr csmv1.ContainerStorageModule, r utils.ReconcileCSM, ctrlClient crclient.Client) error {
	ingress, err := createIngress(isOpenShift, cr)
	if err != nil {
		return fmt.Errorf("creating ingress: %v", err)
	}

	ingressBytes, err := json.Marshal(ingress)
	if err != nil {
		return fmt.Errorf("marshaling ingress: %v", err)
	}

	ingressYaml, err := yaml.JSONToYAML(ingressBytes)
	if err != nil {
		return fmt.Errorf("marshaling ingress: %v", err)
	}

	// Wait for NGINX ingress controller to be ready before creating Ingresses
	// Needed for Kubernetes only
	if !isDeleting && !isOpenShift {
		if err := utils.WaitForNginxController(ctx, cr, r, time.Duration(10)*time.Second); err != nil {
			return fmt.Errorf("NGINX ingress controller is not ready: %v", err)
		}
	}

	err = applyDeleteObjects(ctx, ctrlClient, string(ingressYaml), isDeleting)
	if err != nil {
		return err
	}

	return nil
}

// getNginxIngressController - configure nginx ingress controller with the specified namespace before installation
func getNginxIngressController(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return YamlString, err
	}

	buf, err := readConfigFile(auth, cr, op, AuthNginxIngressManifest)
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	authNamespace := cr.Namespace
	YamlString = strings.ReplaceAll(YamlString, AuthNamespace, authNamespace)
	YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)

	return YamlString, nil
}

// NginxIngressController - apply/delete nginx ingress controller objects
func NginxIngressController(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	YamlString, err := getNginxIngressController(op, cr)
	if err != nil {
		return err
	}

	err = applyDeleteObjects(ctx, ctrlClient, YamlString, isDeleting)
	if err != nil {
		return err
	}

	return nil
}

// getPolicies - configure policies with the specified namespace before installation
func getPolicies(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return YamlString, err
	}

	buf, err := readConfigFile(auth, cr, op, AuthPolicyManifest)
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	authNamespace := cr.Namespace
	YamlString = strings.ReplaceAll(YamlString, AuthNamespace, authNamespace)

	return YamlString, nil
}

// InstallPolicies - apply/delete authorization opa policy objects
func InstallPolicies(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	YamlString, err := getPolicies(op, cr)
	if err != nil {
		return err
	}

	err = applyDeleteObjects(ctx, ctrlClient, YamlString, isDeleting)
	if err != nil {
		return err
	}

	return nil
}

func getCerts(ctx context.Context, op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (bool, string, error) {
	log := logger.GetLogger(ctx)
	YamlString := ""
	authNamespace := cr.Namespace

	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return false, YamlString, err
	}

	for _, component := range authModule.Components {
		if component.Name == AuthProxyServerComponent {
			authHostname = component.Hostname
			authCertificate = component.Certificate
			authPrivateKey = component.PrivateKey
		}
	}

	if authCertificate != "" || authPrivateKey != "" {
		// use custom tls secret
		if authCertificate != "" && authPrivateKey != "" {
			log.Infof("using user provided certificate and key for authorization")
			buf, err := readConfigFile(authModule, cr, op, AuthCustomCert)
			if err != nil {
				return false, YamlString, err
			}

			YamlString = string(buf)
			YamlString = strings.ReplaceAll(YamlString, AuthNamespace, authNamespace)
			YamlString = strings.ReplaceAll(YamlString, AuthCert, authCertificate)
			YamlString = strings.ReplaceAll(YamlString, AuthPrivateKey, authPrivateKey)
		} else {
			return false, YamlString, fmt.Errorf("authorization install failed -- either certificate or private key missing for custom cert")
		}
	} else {
		// use self-signed cert
		log.Info("using self-signed certificate for authorization")
		return true, "", nil
	}

	return false, YamlString, nil
}

// InstallWithCerts - apply/delete certificate related objects
func InstallWithCerts(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	useSelfSignedCert, YamlString, err := getCerts(ctx, op, cr)
	if err != nil {
		return err
	}

	if useSelfSignedCert {
		issuer := createSelfSignedIssuer(cr, "selfsigned")
		issuerByes, err := json.Marshal(issuer)
		if err != nil {
			return fmt.Errorf("marshaling ingress: %v", err)
		}

		issuerYaml, err := yaml.JSONToYAML(issuerByes)
		if err != nil {
			return fmt.Errorf("marshaling ingress: %v", err)
		}

		// create/delete issuer
		err = applyDeleteObjects(ctx, ctrlClient, string(issuerYaml), isDeleting)
		if err != nil {
			return err
		}

		hosts, err := getHosts(cr)
		if err != nil {
			return err
		}

		cert := createSelfSignedCertificate(cr, hosts, "karavi-auth", "karavi-selfsigned-tls", "selfsigned")

		certBytes, err := json.Marshal(cert)
		if err != nil {
			return fmt.Errorf("marshaling ingress: %v", err)
		}

		certYaml, err := yaml.JSONToYAML(certBytes)
		if err != nil {
			return fmt.Errorf("marshaling ingress: %v", err)
		}

		// create/delete certificate
		err = applyDeleteObjects(ctx, ctrlClient, string(certYaml), isDeleting)
		if err != nil {
			return err
		}
	}

	err = applyDeleteObjects(ctx, ctrlClient, YamlString, isDeleting)
	if err != nil {
		return err
	}

	return nil
}

// getAuthCrdDeploy - apply and deploy authorization crd manifest
func getAuthCrdDeploy(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	yamlString := ""

	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return yamlString, err
	}

	buf, err := readConfigFile(auth, cr, op, AuthCrds)
	if err != nil {
		return yamlString, err
	}

	yamlString = string(buf)

	yamlString = strings.ReplaceAll(yamlString, AuthNamespace, cr.Namespace)

	return yamlString, nil
}

// AuthCrdDeploy - apply and delete Auth crds deployment
func AuthCrdDeploy(ctx context.Context, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	// v1 does not have custom resources, so treat it like a no-op
	if semver.Compare(auth.ConfigVersion, "v2.0.0-alpha") < 0 {
		return nil
	}

	yamlString, err := getAuthCrdDeploy(op, cr)
	if err != nil {
		return err
	}

	err = applyDeleteObjects(ctx, ctrlClient, yamlString, false)
	if err != nil {
		return err
	}

	return nil
}

func createSelfSignedIssuer(cr csmv1.ContainerStorageModule, name string) *certificate.Issuer {
	issuer := &certificate.Issuer{
		TypeMeta: metav1.TypeMeta{
			Kind: "Issuer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
		},
		Spec: certificate.IssuerSpec{
			IssuerConfig: certificate.IssuerConfig{
				SelfSigned: &certificate.SelfSignedIssuer{
					CRLDistributionPoints: []string{},
				},
			},
		},
	}

	return issuer
}

func createSelfSignedCertificate(cr csmv1.ContainerStorageModule, hosts []string, name string, secretName string, issuerName string) *certificate.Certificate {
	certificate := &certificate.Certificate{
		TypeMeta: metav1.TypeMeta{
			Kind: "Certificate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
		},
		Spec: certificate.CertificateSpec{
			SecretName: secretName,
			Duration: &metav1.Duration{
				Duration: duration, // 90d
			},
			RenewBefore: &metav1.Duration{
				Duration: renewBefore, // 15d
			},
			Subject: &certificate.X509Subject{
				Organizations: []string{"dellemc"},
			},
			IsCA: false,
			PrivateKey: &certificate.CertificatePrivateKey{
				Algorithm: "RSA",
				Encoding:  "PKCS1",
				Size:      2048,
			},
			Usages: []certificate.KeyUsage{
				"client auth",
				"server auth",
			},
			DNSNames: hosts,
			IssuerRef: cmmetav1.ObjectReference{
				Name:  issuerName,
				Kind:  "Issuer",
				Group: "cert-manager.io",
			},
		},
	}

	return certificate
}

func createIngress(isOpenShift bool, cr csmv1.ContainerStorageModule) (*networking.Ingress, error) {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return nil, err
	}

	className, err := getClassName(isOpenShift, cr)
	if err != nil {
		return nil, fmt.Errorf("getting ingress class name: %v", err)
	}

	annotations, err := getAnnotations(isOpenShift, cr)
	if err != nil {
		return nil, fmt.Errorf("getting annotations: %v", err)
	}

	hosts, err := getHosts(cr)
	if err != nil {
		return nil, fmt.Errorf("getting hosts: %v", err)
	}

	rules, err := setIngressRules(cr)
	if err != nil {
		return nil, fmt.Errorf("setting ingress rules: %v", err)
	}

	for _, component := range authModule.Components {
		if component.Name == AuthProxyServerComponent {
			if component.Certificate != "" && component.PrivateKey != "" {
				secretName = "user-provided-tls"
			} else {
				secretName = "karavi-selfsigned-tls"
			}
		}
	}

	ingress := networking.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind: "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "proxy-server",
			Namespace:   cr.Namespace,
			Annotations: annotations,
		},
		Spec: networking.IngressSpec{
			IngressClassName: &className,
			TLS: []networking.IngressTLS{
				{
					Hosts:      hosts,
					SecretName: secretName,
				},
			},
			Rules: rules,
		},
	}

	return &ingress, nil
}

func getAnnotations(isOpenShift bool, cr csmv1.ContainerStorageModule) (map[string]string, error) {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return nil, err
	}

	annotations := make(map[string]string)

	if isOpenShift {
		annotations["route.openshift.io/termination"] = "edge"
	}

	for _, component := range authModule.Components {
		if component.Name == AuthProxyServerComponent {
			for _, ingress := range component.ProxyServerIngress {
				for annotation, value := range ingress.Annotations {
					annotations[annotation] = value
				}
			}
		}
	}

	return annotations, nil
}

func getHosts(cr csmv1.ContainerStorageModule) ([]string, error) {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return nil, err
	}

	var hosts []string
	for _, component := range authModule.Components {
		if component.Name == AuthProxyServerComponent {
			// hostname
			hosts = append(hosts, component.Hostname)

			for _, proxyServerIngress := range component.ProxyServerIngress {
				// proxyServerIngress.hosts
				hosts = append(hosts, proxyServerIngress.Hosts...)
			}
		}
	}

	return hosts, nil
}

func getClassName(isOpenShift bool, cr csmv1.ContainerStorageModule) (string, error) {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return "", err
	}

	for _, component := range authModule.Components {
		if component.Name == AuthProxyServerComponent {
			for _, proxyServerIngress := range component.ProxyServerIngress {
				if !isOpenShift {
					proxyIngressClassName = proxyServerIngress.IngressClassName
				} else {
					proxyIngressClassName = "openshift-default"
				}
			}
		}
	}

	return proxyIngressClassName, nil
}

func setIngressRules(cr csmv1.ContainerStorageModule) ([]networking.IngressRule, error) {
	var rules []networking.IngressRule
	hosts, err := getHosts(cr)
	if err != nil {
		return nil, fmt.Errorf("getting hosts: %v", err)
	}

	for _, host := range hosts {
		rule := []networking.IngressRule{
			{
				Host: host,
				IngressRuleValue: networking.IngressRuleValue{
					HTTP: &networking.HTTPIngressRuleValue{
						Paths: []networking.HTTPIngressPath{
							{
								Backend: networking.IngressBackend{
									Service: &networking.IngressServiceBackend{
										Name: "proxy-server",
										Port: networking.ServiceBackendPort{
											Number: 8080,
										},
									},
								},
								Path:     "/",
								PathType: &pathType,
							},
						},
					},
				},
			},
		}

		rules = append(rules, rule...)
	}

	noHostRule := []networking.IngressRule{
		{
			// no host specified, uses cluster node IP address
			IngressRuleValue: networking.IngressRuleValue{
				HTTP: &networking.HTTPIngressRuleValue{
					Paths: []networking.HTTPIngressPath{
						{
							Backend: networking.IngressBackend{
								Service: &networking.IngressServiceBackend{
									Name: "proxy-server",
									Port: networking.ServiceBackendPort{
										Number: 8080,
									},
								},
							},
							Path:     "/",
							PathType: &pathType,
						},
					},
				},
			},
		},
	}

	rules = append(rules, noHostRule...)

	return rules, nil
}
