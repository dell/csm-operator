//  Copyright © 2021 - 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
	operatorutils "github.com/dell/csm-operator/pkg/operatorutils"
	"golang.org/x/mod/semver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	// AuthConfigSecretComponent - config secret component
	AuthConfigSecretComponent = "config"
	// AuthVaultComponent - vault component
	AuthVaultComponent = "vault"
	// AuthStorageSystemCredentialsComponent - storage-system-credentials component
	AuthStorageSystemCredentialsComponent = "storage-system-credentials"
	// defaultRedisSecretName - name of default redis K8s secret
	defaultRedisSecretName = "redis-csm-secret" // #nosec G101 -- This is a false positive
	// defaultRedisUsernameKey - name of the default username key
	defaultRedisUsernameKey = "commander_user"
	// defaultRedisPasswordKey - name of default password key
	defaultRedisPasswordKey = "password"
	// defaultConfigSecretName - the default secret name used for the "config-volume" volume
	defaultConfigSecretName = "karavi-config-secret" // #nosec G101 -- This is a false positive

	// AuthLocalStorageClass -
	AuthLocalStorageClass = "csm-authorization-local-storage"

	// AuthCrds - name of authorization crd manifest yaml
	AuthCrds = "authorization-crds.yaml"

	// AuthCSMNameSpace - namespace CSM is found in. Needed for cases where pod namespace is not namespace of CSM
	AuthCSMNameSpace string = "<CSM_NAMESPACE>"
)

var (
	redisStorageClass             string
	redisSecretProviderClassName  string
	redisSecretName               string
	redisUsernameKey              string
	redisPasswordKey              string
	redisConjurUsernamePath       string
	redisConjurPasswordPath       string
	configSecretProviderClassName string
	configSecretName              string
	configSecretPath              string
	authHostname                  string
	proxyIngressClassName         string
	authCertificate               string
	authPrivateKey                string
	secretName                    string

	pathType    = networking.PathTypePrefix
	duration    = 2160 * time.Hour // 90d
	renewBefore = 360 * time.Hour  // 15d
)

// AuthorizationSupportedDrivers is a map containing the CSI Drivers supported by CSM Authorization.
// The key is driver name and the value is the driver plugin identifier.
// powerscale/isilon & powerflex/vxflexos are valid types
var AuthorizationSupportedDrivers = map[string]SupportedDriverParam{
	"powerscale": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
		DriverConfigVolumeMount:       drivers.PowerScaleConfigVolumeMount,
		DriverConfigVolumeMountPath:   drivers.PowerScaleConfigVolumeMountPath,
	},
	"isilon": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
		DriverConfigVolumeMount:       drivers.PowerScaleConfigVolumeMount,
		DriverConfigVolumeMountPath:   drivers.PowerScaleConfigVolumeMountPath,
	},
	"powerflex": {
		PluginIdentifier:              drivers.PowerFlexPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerFlexConfigParamsVolumeMount,
		DriverConfigVolumeMount:       drivers.PowerFlexConfigVolumeMount,
		DriverConfigVolumeMountPath:   drivers.PowerFlexConfigVolumeMountPath,
	},
	"vxflexos": {
		PluginIdentifier:              drivers.PowerFlexPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerFlexConfigParamsVolumeMount,
		DriverConfigVolumeMount:       drivers.PowerFlexConfigVolumeMount,
		DriverConfigVolumeMountPath:   drivers.PowerFlexConfigVolumeMountPath,
	},
	"powermax": {
		PluginIdentifier:              drivers.PowerMaxPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerMaxConfigParamsVolumeMount,
	},
	"powerstore": {
		PluginIdentifier:              drivers.PowerStorePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerStoreConfigParamsVolumeMount,
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

func getAuthApplyCR(cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig) (*csmv1.Module, *acorev1.ContainerApplyConfiguration, error) {
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
		authConfigVersion, err = operatorutils.GetModuleDefaultVersion(cr.Spec.Driver.ConfigVersion, cr.Spec.Driver.CSIDriverType, csmv1.Authorization, op.ConfigDirectory)
		if err != nil {
			return nil, nil, err
		}
	}

	configMapPath := fmt.Sprintf("%s/moduleconfig/authorization/%s/container.yaml", op.ConfigDirectory, authConfigVersion)
	buf, err := os.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		return nil, nil, err
	}

	YamlString := operatorutils.ModifyCommonCR(string(buf), cr)

	YamlString = strings.ReplaceAll(YamlString, DefaultPluginIdentifier, AuthorizationSupportedDrivers[string(cr.Spec.Driver.CSIDriverType)].PluginIdentifier)
	YamlString = strings.ReplaceAll(YamlString, AuthCSMNameSpace, cr.Namespace)

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
	container.Env = operatorutils.ReplaceAllApplyCustomEnvs(container.Env, authModule.Components[0].Envs, authModule.Components[0].Envs)

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

	SupportedDriverParams := AuthorizationSupportedDrivers[string(cr.Spec.Driver.CSIDriverType)]
	for i, c := range container.VolumeMounts {
		switch *c.Name {
		case DefaultDriverConfigParamsVolumeMount:
			newName := SupportedDriverParams.DriverConfigParamsVolumeMount
			container.VolumeMounts[i].Name = &newName
		case DefaultDriverConfigVolumeMount:
			newConfigName := SupportedDriverParams.DriverConfigVolumeMount
			container.VolumeMounts[i].Name = &newConfigName
			newConfigPath := SupportedDriverParams.DriverConfigVolumeMountPath
			container.VolumeMounts[i].MountPath = &newConfigPath
		}
	}

	return &authModule, &container, nil
}

func getAuthApplyVolumes(cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, auth csmv1.ContainerTemplate) ([]acorev1.VolumeApplyConfiguration, error) {
	version, err := operatorutils.GetModuleDefaultVersion(cr.Spec.Driver.ConfigVersion, cr.Spec.Driver.CSIDriverType, csmv1.Authorization, op.ConfigDirectory)
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
func AuthInjectDaemonset(ds applyv1.DaemonSetApplyConfiguration, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig) (*applyv1.DaemonSetApplyConfiguration, error) {
	authModule, containerPtr, err := getAuthApplyCR(cr, op)
	if err != nil {
		return nil, err
	}

	container := *containerPtr
	operatorutils.UpdateSideCarApply(authModule.Components, &container)

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
func AuthInjectDeployment(dp applyv1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig) (*applyv1.DeploymentApplyConfiguration, error) {
	authModule, containerPtr, err := getAuthApplyCR(cr, op)
	if err != nil {
		return nil, err
	}

	container := *containerPtr
	operatorutils.UpdateSideCarApply(authModule.Components, &container)

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
func AuthorizationPrecheck(ctx context.Context, op operatorutils.OperatorConfig, auth csmv1.Module, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
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

	var secrets []string

	// Karavi authorization config is not used in config v2.3.0 and later (CSM 1.15)
	condensedSecretVersion, err := operatorutils.MinVersionCheck("v2.3.0", auth.ConfigVersion)
	if err != nil {
		return err
	}
	if condensedSecretVersion {
		secrets = []string{"proxy-authz-tokens"}
	} else {
		secrets = []string{"karavi-authorization-config", "proxy-authz-tokens"}
	}

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
func AuthorizationServerPrecheck(ctx context.Context, op operatorutils.OperatorConfig, auth csmv1.Module, cr csmv1.ContainerStorageModule, r operatorutils.ReconcileCSM) error {
	log := logger.GetLogger(ctx)

	if auth.ConfigVersion != "" {
		err := checkVersion(string(csmv1.Authorization), auth.ConfigVersion, op.ConfigDirectory)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("authorization version is empty")
	}

	configComponentFound := false
	configSecretProviderClassFound := false
	for _, component := range auth.Components {
		if component.Name == AuthConfigSecretComponent {
			configComponentFound = true
			for _, config := range component.ConfigSecretProviderClass {
				if config.SecretProviderClassName != "" {
					configSecretProviderClassFound = true
				}
			}
		}
	}
	if !configComponentFound || !configSecretProviderClassFound {
		// Check for secrets
		proxyServerSecrets := []string{"karavi-config-secret"}
		for _, name := range proxyServerSecrets {
			found := &corev1.Secret{}
			err := r.GetClient().Get(ctx, types.NamespacedName{Name: name, Namespace: cr.GetNamespace()}, found)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return fmt.Errorf("failed to find secret %s", name)
				}
			}
		}
	}

	log.Infof("preformed pre-checks for %s proxy server", auth.Name)
	return nil
}

// getAuthorizationServerDeployment - apply dynamic values to the deployment manifest before installation
func getAuthorizationServerDeployment(op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
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
			YamlString = strings.ReplaceAll(YamlString, AuthCSMNameSpace, cr.Namespace)
		}

		// redis component
		if component.Name == AuthRedisComponent {
			YamlString = strings.ReplaceAll(YamlString, AuthRedisImage, component.Redis)
			YamlString = strings.ReplaceAll(YamlString, AuthRedisCommanderImage, component.Commander)
			YamlString = strings.ReplaceAll(YamlString, AuthRedisName, component.RedisName)
			YamlString = strings.ReplaceAll(YamlString, AuthRedisCommander, component.RedisCommander)
			YamlString = strings.ReplaceAll(YamlString, AuthRedisSentinel, component.Sentinel)
			YamlString = strings.ReplaceAll(YamlString, AuthRedisReplicas, strconv.Itoa(component.RedisReplicas))
			YamlString = strings.ReplaceAll(YamlString, AuthCSMNameSpace, cr.Namespace)

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

			// create redis kubernetes secret
			for _, config := range component.RedisSecretProviderClass {
				if config.SecretProviderClassName == "" && config.RedisSecretName == "" {
					redisSecret := createRedisK8sSecret(defaultRedisSecretName, cr.Namespace)
					secretYaml, err := yaml.Marshal(redisSecret)
					if err != nil {
						return YamlString, fmt.Errorf("failed to marshal redis kubernetes secret: %w", err)
					}

					YamlString += fmt.Sprintf("\n---\n%s", secretYaml)
				}
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, AuthNamespace, authNamespace)
	YamlString = strings.ReplaceAll(YamlString, AuthRedisStorageClass, redisStorageClass)
	YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)
	YamlString = strings.ReplaceAll(YamlString, AuthCSMNameSpace, cr.Namespace)

	return YamlString, nil
}

// getAuthorizationLocalProvisioner for redis
func getAuthorizationLocalProvisioner(op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) (bool, string, error) {
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
func AuthorizationServerDeployment(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

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

	// scaffolds are applied only for v2.3.0 and above for secret provider class mounts and volumes
	ok, err := operatorutils.MinVersionCheck("v2.3.0", authModule.ConfigVersion)
	if err != nil {
		return err
	}

	if ok {
		err = applyDeleteAuthorizationRedisStatefulsetV2(ctx, isDeleting, cr, ctrlClient)
		if err != nil {
			return err
		}

		err = applyDeleteAuthorizationRediscommanderDeploymentV2(ctx, isDeleting, cr, ctrlClient)
		if err != nil {
			return err
		}

		err = applyDeleteAuthorizationSentinelStatefulsetV2(ctx, isDeleting, cr, ctrlClient)
		if err != nil {
			return err
		}

		err = applyDeleteAuthorizationProxyServerV2(ctx, isDeleting, cr, ctrlClient)
		if err != nil {
			return err
		}

		err = applyDeleteAuthorizationTenantServiceV2(ctx, isDeleting, cr, ctrlClient)
		if err != nil {
			return err
		}
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
	configSecretName = defaultConfigSecretName
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthProxyServerComponent:
			image = component.StorageService
		}
	}

	deployment := getStorageServiceScaffold(cr.Name, cr.Namespace, image, 1, configSecretName)

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
	log := logger.GetLogger(ctx)
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	// SecretProviderClasses and K8s secret for storage credentials is supported from config v2.3.0 (CSM 1.15) onwards
	storageCreds, err := operatorutils.MinVersionCheck("v2.3.0", authModule.ConfigVersion)
	if err != nil {
		return err
	}

	// Vault is supported only till config v2.2.0 (CSM 1.14)
	if !storageCreds {
		err = applyDeleteVaultCertificates(ctx, isDeleting, cr, ctrlClient)
		if err != nil {
			return fmt.Errorf("applying/deleting vault certificates: %w", err)
		}
	}

	replicas := 0
	sentinelName := ""
	redisReplicas := 0
	image := ""
	vaults := []csmv1.Vault{}
	var secretProviderClasses *csmv1.StorageSystemSecretProviderClasses
	var secrets []string
	leaderElection := true
	otelCollector := ""
	configSecretName = defaultConfigSecretName
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthProxyServerComponent:
			replicas = component.StorageServiceReplicas
			image = component.StorageService
			leaderElection = component.LeaderElection
			otelCollector = component.OpenTelemetryCollectorAddress
		case AuthRedisComponent:
			sentinelName = component.Sentinel
			redisReplicas = component.RedisReplicas
			updateRedisGlobalVars(component)
		case AuthVaultComponent:
			vaults = component.Vaults
		case AuthStorageSystemCredentialsComponent:
			secretProviderClasses = component.SecretProviderClasses
			secrets = component.Secrets
		case AuthConfigSecretComponent:
			for _, config := range component.ConfigSecretProviderClass {
				if config.SecretProviderClassName != "" && config.ConfigSecretName != "" {
					configSecretName = config.ConfigSecretName
				}
			}
		default:
			continue
		}
	}

	// Either SecretProviderClasses OR Secrets must be specified (mutually exclusive) from config v2.3.0 (CSM 1.15) onwards
	if storageCreds {
		hasSPC := secretProviderClasses != nil && (len(secretProviderClasses.Vaults) > 0 || len(secretProviderClasses.Conjurs) > 0)
		hasSecrets := len(secrets) > 0

		if hasSPC == hasSecrets {
			return fmt.Errorf("exactly one of SecretProviderClasses or Secrets must be specified in the CSM Authorization CR — not both, not neither")
		}
	}

	// conversion to int32 is safe for a value up to 2147483647
	// #nosec G115
	deployment := getStorageServiceScaffold(cr.Name, cr.Namespace, image, int32(replicas), configSecretName)

	// SecretProviderClasses is supported from config v2.3.0 (CSM 1.15) onwards
	if storageCreds {
		// remove vault from version v2.3.0 since vault is not supported in v2.3.0 and onwards
		err := removeVaultFromStorageService(ctx, cr, ctrlClient, deployment)
		if err != nil {
			return fmt.Errorf("removing vault from storage service: %v", err)
		}

		// Determine whether to read from secret provider classes or kubernetes secrets
		if secretProviderClasses != nil && (len(secretProviderClasses.Vaults) > 0 || len(secretProviderClasses.Conjurs) > 0) {
			log.Info("Using secret provider classes for storage system credentials")
			// set volumes for secret provider classes
			configureSecretProviderClass(secretProviderClasses, &deployment)
		} else {
			log.Info("Using Kubernetes secret for storage system credentials")
			// set volumes for kubernetes secrets
			mountSecretVolumes(secrets, &deployment)
		}

		// redis secret provider class
		if redisSecretProviderClassName != "" && redisSecretName != "" {
			updateConjurAnnotations(deployment.Spec.Template.Annotations, redisConjurUsernamePath, redisConjurPasswordPath)
			mountSPCVolume(&deployment.Spec.Template.Spec, redisSecretProviderClassName)
		}
	} else {
		log.Info("Using Vault for storage system credentials")
		// Vault is supported only till config v2.2.0 (CSM 1.14)

		// apply vault certificates
		err = applyDeleteVaultCertificates(ctx, isDeleting, cr, ctrlClient)
		if err != nil {
			return fmt.Errorf("applying/deleting vault certificates: %w", err)
		}

		// set vault volumes
		log.Infof("Using Vault for storage system credentials")
		mountVaultVolumes(vaults, &deployment)
	}

	// set redis envs
	redis := []corev1.EnvVar{
		{
			Name:  "SENTINELS",
			Value: buildSentinelList(redisReplicas, sentinelName, cr.Namespace),
		},
		{
			Name: "REDIS_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: redisSecretName,
					},
					Key: redisPasswordKey,
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

	// Vault is supported only till config v2.2.0 (CSM 1.14)
	var vaultArgs []string
	if !storageCreds {
		for _, vault := range vaults {
			vaultArgs = append(vaultArgs, fmt.Sprintf("--vault=%s,%s,%s,%t", vault.Identifier, vault.Address, vault.Role, vault.SkipCertificateValidation))
		}
	}

	// set arguments
	args := []string{
		"--redis-sentinel=$(SENTINELS)",
		"--redis-password=$(REDIS_PASSWORD)",
		fmt.Sprintf("--leader-election=%t", leaderElection),
	}

	// if the config version is greater than v2.0.0-alpha, add the collector-address arg
	v2Version, err := operatorutils.MinVersionCheck("v2.0.0", authModule.ConfigVersion)
	if err != nil {
		return err
	}
	if v2Version {
		args = append(args, fmt.Sprintf("--collector-address=%s", otelCollector))
	}

	for i, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "storage-service" {
			deployment.Spec.Template.Spec.Containers[i].Args = append(deployment.Spec.Template.Spec.Containers[i].Args, args...)
			break
		}
	}

	// if the config version is greater than v2.0.0-alpha, set promhttp container port
	if v2Version {
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

// remove vault certificates, args, and volumes/volume mounts if upgrading from verions < v2.3.0
func removeVaultFromStorageService(ctx context.Context, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client, dp appsv1.Deployment) error {
	log := logger.GetLogger(ctx)

	currentDeployment := &appsv1.Deployment{}

	// check if there is an existing storage service deployment to be updated
	err := ctrlClient.Get(ctx, client.ObjectKey{
		Namespace: dp.Namespace,
		Name:      dp.Name,
	}, currentDeployment)
	if err != nil {
		log.Infof("%s not found. No need to remove vault from storage service.", dp.Name)
		return nil
	}

	// remove vault certificates
	err = applyDeleteVaultCertificates(ctx, true, cr, ctrlClient)
	if err != nil {
		return fmt.Errorf("deleting vault certificates: %w", err)
	}

	// remove vault args and volume mounts from the deployment's container
	for i, container := range currentDeployment.Spec.Template.Spec.Containers {
		if container.Name == "storage-service" {
			// Filter out vault args
			var newArgs []string
			for _, arg := range container.Args {
				if !strings.HasPrefix(arg, "--vault=") {
					newArgs = append(newArgs, arg)
				}
			}
			currentDeployment.Spec.Template.Spec.Containers[i].Args = newArgs

			// Filter out vault volume mounts
			var newVolumeMounts []corev1.VolumeMount
			for _, volumeMount := range container.VolumeMounts {
				if !strings.Contains(volumeMount.MountPath, "/etc/vault/") {
					newVolumeMounts = append(newVolumeMounts, volumeMount)
				}
			}
			currentDeployment.Spec.Template.Spec.Containers[i].VolumeMounts = newVolumeMounts
		}
	}

	// filter out vault volumes
	var newVolumes []corev1.Volume
	for _, volume := range currentDeployment.Spec.Template.Spec.Volumes {
		if !strings.Contains(volume.Name, "vault-client-certificate-") {
			volume.VolumeSource.Projected = nil // Clear projected sources if they exists
			newVolumes = append(newVolumes, volume)
		}
	}
	currentDeployment.Spec.Template.Spec.Volumes = newVolumes

	// update the storage-service deployment
	err = ctrlClient.Update(ctx, currentDeployment)
	if err != nil {
		return fmt.Errorf("updating storage service deployment for upgrading: %w", err)
	}

	return nil
}

func mountVaultVolumes(vaults []csmv1.Vault, deployment *appsv1.Deployment) {
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
}

func mountSecretVolumes(secrets []string, deployment *appsv1.Deployment) {
	for _, secret := range secrets {
		volume := corev1.Volume{
			Name: fmt.Sprintf("storage-system-secrets-%s", secret),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret,
				},
			},
		}

		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, volume)
	}

	// set volume mounts for kubernetes secrets
	for i, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "storage-service" {
			for _, secret := range secrets {
				deployment.Spec.Template.Spec.Containers[i].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
					Name:      fmt.Sprintf("storage-system-secrets-%s", secret),
					MountPath: fmt.Sprintf("/etc/csm-authorization/%s", secret),
					ReadOnly:  true,
				})
			}
			break
		}
	}
}

// configureSecretProviderClass configures the secret provider class volumes, mounts, and annotations in the deployment
func configureSecretProviderClass(secretProviderClasses *csmv1.StorageSystemSecretProviderClasses, deployment *appsv1.Deployment) {
	configureVaultSecretProvider(secretProviderClasses, deployment)
	configureConjurSecretProvider(secretProviderClasses, deployment)
}

func configureVaultSecretProvider(secretProviderClasses *csmv1.StorageSystemSecretProviderClasses, deployment *appsv1.Deployment) {
	readOnly := true
	for _, vault := range secretProviderClasses.Vaults {
		volume := corev1.Volume{
			Name: fmt.Sprintf("secrets-store-inline-%s", vault),
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:   "secrets-store.csi.k8s.io",
					ReadOnly: &readOnly,
					VolumeAttributes: map[string]string{
						"secretProviderClass": vault,
					},
				},
			},
		}

		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, volume)
	}

	for i, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "storage-service" {
			for _, vault := range secretProviderClasses.Vaults {
				deployment.Spec.Template.Spec.Containers[i].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
					Name:      fmt.Sprintf("secrets-store-inline-%s", vault),
					MountPath: fmt.Sprintf("/etc/csm-authorization/%s", vault),
					ReadOnly:  true,
				})
			}
		}
	}
}

func configureConjurSecretProvider(secretProviderClasses *csmv1.StorageSystemSecretProviderClasses, deployment *appsv1.Deployment) {
	readOnly := true
	for _, conjur := range secretProviderClasses.Conjurs {
		volume := corev1.Volume{
			Name: fmt.Sprintf("secrets-store-inline-%s", conjur.Name),
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:   "secrets-store.csi.k8s.io",
					ReadOnly: &readOnly,
					VolumeAttributes: map[string]string{
						"secretProviderClass": conjur.Name,
					},
				},
			},
		}

		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, volume)
	}

	for i, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "storage-service" {
			annotationFormat := "- %s: %s"
			var secretStringBuilder strings.Builder
			for _, conjur := range secretProviderClasses.Conjurs {
				deployment.Spec.Template.Spec.Containers[i].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
					Name:      fmt.Sprintf("secrets-store-inline-%s", conjur.Name),
					MountPath: fmt.Sprintf("/etc/csm-authorization/%s", conjur.Name),
					ReadOnly:  true,
				})

				for _, path := range conjur.Paths {
					if secretStringBuilder.String() != "" {
						secretStringBuilder.WriteString("\n")
					}
					secretStringBuilder.WriteString(fmt.Sprintf(annotationFormat, path.UsernamePath, path.UsernamePath))
					secretStringBuilder.WriteString("\n")
					secretStringBuilder.WriteString(fmt.Sprintf(annotationFormat, path.PasswordPath, path.PasswordPath))
				}
			}

			if secretPaths := secretStringBuilder.String(); secretPaths != "" {
				annotations := deployment.Spec.Template.ObjectMeta.Annotations
				if annotations == nil {
					annotations = make(map[string]string)
				}
				annotations["conjur.org/secrets"] = secretPaths
				deployment.Spec.Template.ObjectMeta.Annotations = annotations
			}
			break
		}
	}
}

// remove vault certificates, args, and volumes/volume mounts if upgrading from verions < v2.3.0
func removeVaultFromStorageService(ctx context.Context, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client, dp *appsv1.Deployment) error {
	log := logger.GetLogger(ctx)

	// check if there is an existing storage service deployment to be updated
	err := ctrlClient.Get(ctx, client.ObjectKey{
		Namespace: dp.Namespace,
		Name:      dp.Name,
	}, &cr)
	if err != nil {
		log.Infof("%s not found. No need to remvoe vault from storage service.", cr.Name)
		return nil
	}

	// remove vault certificates
	err = applyDeleteVaultCertificates(ctx, true, cr, ctrlClient)
	if err != nil {
		return fmt.Errorf("deleting vault certificates: %w", err)
	}

	// remove vault args and volume mounts from the deployment's container
	for i, container := range dp.Spec.Template.Spec.Containers {
		if container.Name == "storage-service" {
			// Filter out vault args
			var newArgs []string
			for _, arg := range container.Args {
				if !strings.HasPrefix(arg, "--vault=") {
					newArgs = append(newArgs, arg)
				}
			}
			dp.Spec.Template.Spec.Containers[i].Args = newArgs

			// Filter out vault volume mounts
			var newVolumeMounts []corev1.VolumeMount
			for _, volumeMount := range container.VolumeMounts {
				if !strings.Contains(volumeMount.MountPath, "/etc/vault/") {
					newVolumeMounts = append(newVolumeMounts, volumeMount)
				}
			}
			dp.Spec.Template.Spec.Containers[i].VolumeMounts = newVolumeMounts
		}
	}

	// Filter out vault volumes
	var newVolumes []corev1.Volume
	for _, volume := range dp.Spec.Template.Spec.Volumes {
		if !strings.Contains(volume.Name, "vault-client-certificate-") {
			volume.VolumeSource.Projected = nil // Clear projected sources if they exists
			newVolumes = append(newVolumes, volume)
		}
	}
	dp.Spec.Template.Spec.Volumes = newVolumes

	// Update the deployment
	err = ctrlClient.Update(ctx, dp)
	if err != nil {
		return fmt.Errorf("updating storage service deployment for upgrading: %w", err)
	}

	return nil
}

func applyDeleteAuthorizationProxyServerV2(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	replicas := 0
	redisReplicas := 0
	sentinelName := ""
	proxyImage := ""
	opaImage := ""
	opaKubeMgmtImage := ""
	configSecretName = defaultConfigSecretName
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthProxyServerComponent:
			replicas = component.ProxyServiceReplicas
			proxyImage = component.ProxyService
			opaImage = component.Opa
			opaKubeMgmtImage = component.OpaKubeMgmt
		case AuthRedisComponent:
			sentinelName = component.Sentinel
			redisReplicas = component.RedisReplicas
			updateRedisGlobalVars(component)
		case AuthConfigSecretComponent:
			updateConfigGlobalVars(component)
		default:
			continue
		}
	}

	// conversion to int32 is safe for a value up to 2147483647
	// #nosec G115
	deployment := getProxyServerScaffold(cr.Name, sentinelName, cr.Namespace, proxyImage, opaImage, opaKubeMgmtImage, configSecretName, redisSecretName, redisPasswordKey, int32(replicas), redisReplicas)

	if redisSecretProviderClassName != "" && redisSecretName != "" {
		updateConjurAnnotations(deployment.Spec.Template.Annotations, redisConjurUsernamePath, redisConjurPasswordPath)
		mountSPCVolume(&deployment.Spec.Template.Spec, redisSecretProviderClassName)
	}

	if configSecretProviderClassName != "" && configSecretName != "" {
		updateConjurAnnotations(deployment.Spec.Template.Annotations, configSecretPath)
		mountSPCVolume(&deployment.Spec.Template.Spec, configSecretProviderClassName)
	}

	deploymentBytes, err := yaml.Marshal(&deployment)
	if err != nil {
		return fmt.Errorf("marshalling proxy-server deployment: %w", err)
	}

	err = applyDeleteObjects(ctx, ctrlClient, string(deploymentBytes), isDeleting)
	if err != nil {
		return fmt.Errorf("applying proxy-server deployment: %w", err)
	}
	return nil
}

func applyDeleteAuthorizationTenantServiceV2(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	replicas := 0
	redisReplicas := 0
	image := ""
	sentinelName := ""
	configSecretName = defaultConfigSecretName
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthProxyServerComponent:
			image = component.TenantService
			replicas = component.TenantServiceReplicas
		case AuthRedisComponent:
			sentinelName = component.Sentinel
			redisReplicas = component.RedisReplicas
			updateRedisGlobalVars(component)
		case AuthConfigSecretComponent:
			updateConfigGlobalVars(component)
		default:
			continue
		}
	}

	// conversion to int32 is safe for a value up to 2147483647
	// #nosec G115
	deployment := getTenantServiceScaffold(cr.Name, cr.Namespace, sentinelName, image, configSecretName, redisSecretName, redisPasswordKey, int32(replicas), redisReplicas)

	if redisSecretProviderClassName != "" && redisSecretName != "" {
		updateConjurAnnotations(deployment.Spec.Template.Annotations, redisConjurUsernamePath, redisConjurPasswordPath)
		mountSPCVolume(&deployment.Spec.Template.Spec, redisSecretProviderClassName)
	}

	if configSecretProviderClassName != "" && configSecretName != "" {
		updateConjurAnnotations(deployment.Spec.Template.Annotations, configSecretPath)
		mountSPCVolume(&deployment.Spec.Template.Spec, configSecretProviderClassName)
	}

	deploymentBytes, err := yaml.Marshal(&deployment)
	if err != nil {
		return fmt.Errorf("marshalling tenant-service deployment: %w", err)
	}

	err = applyDeleteObjects(ctx, ctrlClient, string(deploymentBytes), isDeleting)
	if err != nil {
		return fmt.Errorf("applying tenant-service deployment: %w", err)
	}
	return nil
}

func applyDeleteAuthorizationRedisStatefulsetV2(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	redisName := ""
	image := ""
	redisReplicas := 0
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthRedisComponent:
			redisName = component.RedisName
			image = component.Redis
			redisReplicas = component.RedisReplicas
			updateRedisGlobalVars(component)
		default:
			continue
		}
	}

	checksum, err := getRedisChecksumFromSecretData(ctx, ctrlClient, cr, redisSecretName)
	if err != nil {
		return fmt.Errorf("getting redis secret checksum: %w", err)
	}

	// conversion to int32 is safe for a value up to 2147483647
	// #nosec G115
	statefulset := getAuthorizationRedisStatefulsetScaffold(cr.Name, redisName, cr.Namespace, image, redisSecretName, redisPasswordKey, checksum, int32(redisReplicas))

	if redisSecretProviderClassName != "" && redisSecretName != "" {
		updateConjurAnnotations(statefulset.Spec.Template.Annotations, redisConjurUsernamePath, redisConjurPasswordPath)
		mountSPCVolume(&statefulset.Spec.Template.Spec, redisSecretProviderClassName)
	}

	statefulsetBytes, err := yaml.Marshal(&statefulset)
	if err != nil {
		return fmt.Errorf("marshalling redis statefulset: %w", err)
	}

	err = applyDeleteObjects(ctx, ctrlClient, string(statefulsetBytes), isDeleting)
	if err != nil {
		return fmt.Errorf("applying redis statefulset: %w", err)
	}
	return nil
}

func applyDeleteAuthorizationRediscommanderDeploymentV2(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	rediscommanderName := ""
	sentinelName := ""
	image := ""
	redisReplicas := 0
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthRedisComponent:
			rediscommanderName = component.RedisCommander
			sentinelName = component.Sentinel
			image = component.Commander
			redisReplicas = component.RedisReplicas
			updateRedisGlobalVars(component)
		default:
			continue
		}
	}

	checksum, err := getRedisChecksumFromSecretData(ctx, ctrlClient, cr, redisSecretName)
	if err != nil {
		return fmt.Errorf("getting redis secret checksum: %w", err)
	}

	// conversion to int32 is safe for a value up to 2147483647
	// #nosec G115
	deployment := getAuthorizationRediscommanderDeploymentScaffold(cr.Name, rediscommanderName, cr.Namespace, image, redisSecretName, redisUsernameKey, redisPasswordKey, sentinelName, checksum, redisReplicas)

	if redisSecretProviderClassName != "" && redisSecretName != "" {
		updateConjurAnnotations(deployment.Spec.Template.Annotations, redisConjurUsernamePath, redisConjurPasswordPath)
		mountSPCVolume(&deployment.Spec.Template.Spec, redisSecretProviderClassName)
	}

	deploymentBytes, err := yaml.Marshal(&deployment)
	if err != nil {
		return fmt.Errorf("marshalling rediscommander deployment: %w", err)
	}

	err = applyDeleteObjects(ctx, ctrlClient, string(deploymentBytes), isDeleting)
	if err != nil {
		return fmt.Errorf("applying rediscommander deployment: %w", err)
	}
	return nil
}

func applyDeleteAuthorizationSentinelStatefulsetV2(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	sentinelName := ""
	redisName := ""
	image := ""
	redisReplicas := 0
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthRedisComponent:
			sentinelName = component.Sentinel
			redisName = component.RedisName
			image = component.Redis
			redisReplicas = component.RedisReplicas
			updateRedisGlobalVars(component)
		default:
			continue
		}
	}

	checksum, err := getRedisChecksumFromSecretData(ctx, ctrlClient, cr, redisSecretName)
	if err != nil {
		return fmt.Errorf("getting redis secret checksum: %w", err)
	}

	// conversion to int32 is safe for a value up to 2147483647
	// #nosec G115
	statefulset := getAuthorizationSentinelStatefulsetScaffold(cr.Name, sentinelName, redisName, cr.Namespace, image, redisSecretName, redisPasswordKey, checksum, int32(redisReplicas))

	if redisSecretProviderClassName != "" && redisSecretName != "" {
		updateConjurAnnotations(statefulset.Spec.Template.Annotations, redisConjurUsernamePath, redisConjurPasswordPath)
		mountSPCVolume(&statefulset.Spec.Template.Spec, redisSecretProviderClassName)
	}

	statefulsetBytes, err := yaml.Marshal(&statefulset)
	if err != nil {
		return fmt.Errorf("marshalling sentinel statefulset: %w", err)
	}

	err = applyDeleteObjects(ctx, ctrlClient, string(statefulsetBytes), isDeleting)
	if err != nil {
		return fmt.Errorf("applying sentinel statefulset: %w", err)
	}
	return nil
}

// mountSPCVolume mounts redis volumes for an authorization deployment or statefulset
func mountSPCVolume(spec *corev1.PodSpec, secretProviderClassName string) {
	mountPath := fmt.Sprintf("/etc/csm-authorization/%s", secretProviderClassName)
	volumeName := fmt.Sprintf("secrets-store-inline-%s", secretProviderClassName)
	readOnly := true

	// check if volume already exists
	volumeExists := false
	for _, volume := range spec.Volumes {
		if volume.Name == volumeName {
			volumeExists = true
			break
		}
	}

	// check if volume mount already exists
	mountExists := false
	for _, container := range spec.Containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == volumeName && mount.MountPath == mountPath {
				mountExists = true
				break
			}
		}

		if mountExists {
			break
		}
	}

	// add volume for redis secret provider class
	if !volumeExists {
		volume := corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:   "secrets-store.csi.k8s.io",
					ReadOnly: &readOnly,
					VolumeAttributes: map[string]string{
						"secretProviderClass": secretProviderClassName,
					},
				},
			},
		}
		spec.Volumes = append(spec.Volumes, volume)
	}

	// set volume mount for redis secret provider class
	if !mountExists {
		for i := range spec.Containers {
			volumeMount := corev1.VolumeMount{
				Name:      volumeName,
				MountPath: mountPath,
				ReadOnly:  true,
			}
			spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts, volumeMount)
		}
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
func AuthorizationIngress(ctx context.Context, isDeleting, isOpenShift bool, cr csmv1.ContainerStorageModule, r operatorutils.ReconcileCSM, ctrlClient crclient.Client) error {
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
		if err := operatorutils.WaitForNginxController(ctx, cr, r, time.Duration(10)*time.Second); err != nil {
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
func getNginxIngressController(op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
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
	YamlString = strings.ReplaceAll(YamlString, AuthCSMNameSpace, cr.Namespace)

	return YamlString, nil
}

// NginxIngressController - apply/delete nginx ingress controller objects
func NginxIngressController(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
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
func getPolicies(op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
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
func InstallPolicies(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
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

func getCerts(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) (bool, string, error) {
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

			log.Infof("Authorization hostname: %s", authHostname)
		}
	}

	if authCertificate != "" || authPrivateKey != "" {
		// use custom tls secret
		if authCertificate != "" && authPrivateKey != "" {
			log.Info("using user provided certificate and key for authorization")
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
func InstallWithCerts(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
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
func getAuthCrdDeploy(op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
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
	yamlString = strings.ReplaceAll(yamlString, AuthCSMNameSpace, cr.Namespace)

	return yamlString, nil
}

// AuthCrdDeploy - apply and delete Auth crds deployment
func AuthCrdDeploy(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	// v1 does not have custom resources, so treat it like a no-op
	if ok, err := operatorutils.MinVersionCheck(auth.ConfigVersion, "v2.0.0-alpha"); !ok {
		return nil
	} else if err != nil {
		return err
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
	return &certificate.Issuer{
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
}

func createSelfSignedCertificate(cr csmv1.ContainerStorageModule, hosts []string, name string, secretName string, issuerName string) *certificate.Certificate {
	return &certificate.Certificate{
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

func getRedisChecksumFromSecretData(ctx context.Context, ctrlClient crclient.Client, cr csmv1.ContainerStorageModule, secretName string) (string, error) {
	log := logger.GetLogger(ctx)
	redisSecret := &corev1.Secret{}

	err := ctrlClient.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: cr.GetNamespace(),
	}, redisSecret)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Infof("Redis secret %s not found; assuming it was deleted.", secretName)
			return "", nil
		}
		log.Warn(err, "Failed to query for redis secret, it could have been deleted.")
		return "", err
	}

	rendered := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      redisSecret.Name,
			Namespace: redisSecret.Namespace,
		},
		Type:       redisSecret.Type,
		StringData: map[string]string{},
	}

	for key, val := range redisSecret.Data {
		rendered.StringData[key] = string(val)
	}

	yamlBytes, err := yaml.Marshal(rendered)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(yamlBytes)
	return hex.EncodeToString(hash[:]), nil
}

// updateRedisGlobalVars - update the global redis vars from the config
func updateRedisGlobalVars(component csmv1.ContainerTemplate) {
	redisSecretProviderClassName = ""
	redisSecretName = defaultRedisSecretName
	redisUsernameKey = defaultRedisUsernameKey
	redisPasswordKey = defaultRedisPasswordKey
	redisConjurUsernamePath = ""
	redisConjurPasswordPath = ""

	for _, config := range component.RedisSecretProviderClass {
		if config.SecretProviderClassName != "" && config.RedisSecretName != "" {
			redisSecretProviderClassName = config.SecretProviderClassName
			redisSecretName = config.RedisSecretName
			redisUsernameKey = config.RedisUsernameKey
			redisPasswordKey = config.RedisPasswordKey
		}

		if config.Conjur != nil {
			redisConjurUsernamePath = config.Conjur.UsernamePath
			redisConjurPasswordPath = config.Conjur.PasswordPath
		}
	}
}

// updateConfigGlobalVars - update the global config vars from the config secret provider class
func updateConfigGlobalVars(component csmv1.ContainerTemplate) {
	configSecretName = defaultConfigSecretName
	configSecretProviderClassName = ""
	configSecretPath = ""
	for _, config := range component.ConfigSecretProviderClass {
		if config.SecretProviderClassName != "" && config.ConfigSecretName != "" {
			configSecretProviderClassName = config.SecretProviderClassName
			configSecretName = config.ConfigSecretName
		}

		if config.Conjur != nil {
			configSecretPath = config.Conjur.SecretPath
		}
	}
}

// updateConjurAnnotations - update the annotations with conjur paths
func updateConjurAnnotations(annotations map[string]string, paths ...string) {
	if len(paths) == 0 {
		return
	}
	for _, path := range paths {
		if path == "" {
			return
		}
	}

	annotationFormat := "- %s: %s"
	var sb strings.Builder

	if v, ok := annotations["conjur.org/secrets"]; ok {
		sb.WriteString(v)
		sb.WriteString("\n")
	}

	var lines []string
	for _, path := range paths {
		lines = append(lines, fmt.Sprintf(annotationFormat, path, path))
	}
	sb.WriteString(strings.Join(lines, "\n"))

	annotations["conjur.org/secrets"] = sb.String()
}
