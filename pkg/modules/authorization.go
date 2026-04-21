//  Copyright © 2021 - 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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

	csmv1 "eos2git.cec.lab.emc.com/CSM/csm-operator/api/v1"
	drivers "eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/drivers"
	"eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/logger"
	operatorutils "eos2git.cec.lab.emc.com/CSM/csm-operator/pkg/operatorutils"
	certificate "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/yaml"
)

var resolveVersionFromConfigMapAuth = operatorutils.ResolveVersionFromConfigMap

const (
	// AuthDeploymentManifest - deployment resources and ingress rules for authorization module
	AuthDeploymentManifest = "deployment.yaml"
	// AuthIngressManifest -
	AuthIngressManifest = "ingress.yaml"
	// AuthCertManagerManifest -
	AuthCertManagerManifest = "cert-manager.yaml"
	// AuthNginxIngressManifest -
	AuthNginxIngressManifest = "nginx-ingress-controller.yaml"
	// AuthGatewayManifest - gateway API controller manifest for authorization module (v2.5.0+)
	AuthGatewayManifest = "gateway-api-controller.yaml"
	// AuthPolicyManifest -
	AuthPolicyManifest = "policies.yaml"
	// AuthCustomCert - custom certificate file
	AuthCustomCert = "custom-cert.yaml"

	// AuthNamespace -
	AuthNamespace = "<NAMESPACE>"

	// Below are variables used in the deployment.yaml file
	// AuthRoleServiceImage -
	AuthRoleServiceImage = "<AUTHORIZATION_ROLE_SERVICE_IMAGE>"
	// AuthRoleServiceReplicas -
	AuthRoleServiceReplicas = "<AUTHORIZATION_ROLE_SERVICE_REPLICAS>"
	// AuthControllerImage -
	AuthControllerImage = "<AUTHORIZATION_CONTROLLER_IMAGE>"
	// AuthControllerReplicas -
	AuthControllerReplicas = "<AUTHORIZATION_CONTROLLER_REPLICAS>"
	// AuthLeaderElectionEnabled -
	AuthLeaderElectionEnabled = "<AUTHORIZATION_LEADER_ELECTION_ENABLED>"
	// AuthControllerReconcileInterval -
	AuthControllerReconcileInterval = "<AUTHORIZATION_CONTROLLER_RECONCILE_INTERVAL>"

	DefaultProxyServerImage    = "quay.io/dell/container-storage-modules/csm-authorization-proxy"
	DefaultOpaImage            = "docker.io/openpolicyagent/opa:0.70.0"
	DefaultOpaKubeMgmtImage    = "docker.io/openpolicyagent/kube-mgmt:9.2.1"
	DefaultTenantServiceImage  = "quay.io/dell/container-storage-modules/csm-authorization-tenant"
	DefaultRoleServiceImage    = "quay.io/dell/container-storage-modules/csm-authorization-role"
	DefaultStorageServiceImage = "quay.io/dell/container-storage-modules/csm-authorization-storage"
	DefaultRedisImage          = "redis:8.4.0-alpine"
	DefaultRedisCommanderImage = "rediscommander/redis-commander:latest"
	DefaultControllerImage     = "quay.io/dell/container-storage-modules/csm-authorization-controller"

	// AuthRedisName -
	AuthRedisName = "<AUTHORIZATION_REDIS_NAME>"
	// AuthRedisCommander -
	AuthRedisCommander = "<AUTHORIZATION_REDIS_COMMANDER>"
	// AuthRedisSentinel -
	AuthRedisSentinel = "<AUTHORIZATION_REDIS_SENTINEL>"
	// AuthCert - for tls secret
	AuthCert = "<BASE64_CERTIFICATE>"
	// AuthPrivateKey - for tls secret
	AuthPrivateKey = "<BASE64_PRIVATE_KEY>"

	// AuthProxyServerComponent - proxy-server component
	AuthProxyServerComponent = "proxy-server"
	// AuthSidecarComponent - karavi-authorization-proxy component
	AuthSidecarComponent = "karavi-authorization-proxy"
	// AuthNginxIngressComponent - nginx ingress component (v2.4.0 and below)
	AuthNginxIngressComponent = "nginx"
	// AuthGatewayComponent - Gateway API controller component name (v2.5.0+)
	AuthGatewayComponent = "nginx-gateway-fabric"
	// AuthCertManagerComponent - cert-manager component
	AuthCertManagerComponent = "cert-manager"
	// AuthRedisComponent - redis component
	AuthRedisComponent = "redis"
	// AuthConfigSecretComponent - config secret component
	AuthConfigSecretComponent = "config"
	// AuthVaultComponent - vault component
	// Removed in v2.3.0 but kept for backwards compatibility
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

	// AuthCrds - name of authorization crd manifest yaml
	AuthCrds = "authorization-crds.yaml"

	// AuthCSMNameSpace - namespace CSM is found in. Needed for cases where pod namespace is not namespace of CSM
	AuthCSMNameSpace string = "<CSM_NAMESPACE>"

	// Karavi authorization config secret name
	// removed in 2.4.0, but still supporting backward compatibility
	KaraviAuthorizationConfigSecret = "karavi-authorization-config"
)

var (
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
	},
	"isilon": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
		DriverConfigVolumeMount:       drivers.PowerScaleConfigVolumeMount,
	},
	"powerflex": {
		PluginIdentifier:              drivers.PowerFlexPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerFlexConfigParamsVolumeMount,
		DriverConfigVolumeMount:       drivers.PowerFlexConfigVolumeMount,
	},
	"vxflexos": {
		PluginIdentifier:              drivers.PowerFlexPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerFlexConfigParamsVolumeMount,
		DriverConfigVolumeMount:       drivers.PowerFlexConfigVolumeMount,
	},
	"powermax": {
		PluginIdentifier:              drivers.PowerMaxPluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerMaxConfigParamsVolumeMount,
		DriverConfigVolumeMount:       drivers.PowerMaxConfigVolumeMount,
	},
	"powerstore": {
		PluginIdentifier:              drivers.PowerStorePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerStoreConfigParamsVolumeMount,
		DriverConfigVolumeMount:       drivers.PowerStoreConfigVolumeMount,
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
func CheckApplyVolumesAuth(volumes []acorev1.VolumeApplyConfiguration, authConfigVersion string, drivertype string, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	// Karavi authorization config is not required in config v2.4.0 and later (CSM 1.16) due to driver secret being used instead if the config is not present
	volumeNames := []string{}
	driverSecretVersion, err := operatorutils.MinVersionCheck("v2.4.0", authConfigVersion)
	if err != nil {
		return fmt.Errorf("error checking version: %s", authConfigVersion)
	}
	if driverSecretVersion {
		// If the karavi-authorization-config secret exists, check for that mount, otherwise check for the driver secret.
		_, err := operatorutils.GetSecret(context.TODO(), KaraviAuthorizationConfigSecret, cr.GetNamespace(), ctrlClient)
		if err != nil {
			volumeNames = append(volumeNames, AuthorizationSupportedDrivers[drivertype].DriverConfigVolumeMount)
		} else {
			volumeNames = append(volumeNames, KaraviAuthorizationConfigSecret)
		}
	} else {
		volumeNames = append(volumeNames, KaraviAuthorizationConfigSecret)
	}

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
func CheckApplyContainersAuth(containers []acorev1.ContainerApplyConfiguration, drivertype string, skipCertificateValidation bool, authConfigVersion string, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	// Karavi authorization config is not required in config v2.4.0 and later (CSM 1.16) due to driver secret being used instead if the config is not present
	volumeMounts := []string{AuthorizationSupportedDrivers[drivertype].DriverConfigParamsVolumeMount}
	driverSecretVersion, err := operatorutils.MinVersionCheck("v2.4.0", authConfigVersion)
	if err != nil {
		return fmt.Errorf("error checking version: %s", authConfigVersion)
	}
	if driverSecretVersion {
		// If the karavi-authorization-config secret exists, check for that mount, otherwise check for the driver secret.
		_, err := operatorutils.GetSecret(context.TODO(), KaraviAuthorizationConfigSecret, cr.GetNamespace(), ctrlClient)
		if err != nil {
			volumeMounts = append(volumeMounts, AuthorizationSupportedDrivers[drivertype].DriverConfigVolumeMount)
		} else {
			volumeMounts = append(volumeMounts, KaraviAuthorizationConfigSecret)
		}
	} else {
		volumeMounts = append(volumeMounts, KaraviAuthorizationConfigSecret)
	}

	authString := "karavi-authorization-proxy"
	for _, cnt := range containers {
		if *cnt.Name == authString {
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

func getAuthApplyCR(ctx context.Context, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, ctrlClient crclient.Client) (*csmv1.Module, *acorev1.ContainerApplyConfiguration, operatorutils.VersionSpec, error) {
	log := logger.GetLogger(ctx)
	var err error
	authModule := csmv1.Module{}
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Authorization {
			authModule = m
			break
		}
	}

	emptySpec := operatorutils.VersionSpec{}
	authConfigVersion := authModule.ConfigVersion
	if authConfigVersion == "" {
		version, err := operatorutils.GetVersion(ctx, &cr, op)
		if err != nil {
			return nil, nil, emptySpec, err
		}
		authConfigVersion, err = operatorutils.GetModuleDefaultVersion(version, cr.Spec.Driver.CSIDriverType, csmv1.Authorization, op.ConfigDirectory)
		if err != nil {
			return nil, nil, emptySpec, err
		}
	}

	configMapPath := fmt.Sprintf("%s/moduleconfig/authorization/%s/container.yaml", op.ConfigDirectory, authConfigVersion)
	buf, err := os.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		return nil, nil, emptySpec, err
	}

	YamlString := operatorutils.ModifyCommonCR(string(buf), cr)

	YamlString = strings.ReplaceAll(YamlString, DefaultPluginIdentifier, AuthorizationSupportedDrivers[string(cr.Spec.Driver.CSIDriverType)].PluginIdentifier)
	YamlString = strings.ReplaceAll(YamlString, AuthCSMNameSpace, cr.Namespace)

	var container acorev1.ContainerApplyConfiguration
	err = yaml.Unmarshal([]byte(YamlString), &container)
	if err != nil {
		return nil, nil, emptySpec, err
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

	// Karavi authorization config is not required in config v2.4.0 and later (CSM 1.16) due to driver secret
	driverSecretVersion, err := operatorutils.MinVersionCheck("v2.4.0", authConfigVersion)
	if err != nil {
		return nil, nil, emptySpec, err
	}
	if driverSecretVersion {
		// Do not try to make the karavi-authorization-config volume available if the customer is using the driver secret.
		_, err := operatorutils.GetSecret(ctx, KaraviAuthorizationConfigSecret, cr.GetNamespace(), ctrlClient)
		if err != nil {
			for i, c := range container.VolumeMounts {
				if *c.Name == KaraviAuthorizationConfigSecret {
					driverSecretNamePlaceholder := "<DriverConfigVolumeMount>" // #nosec G101 -- This is a false positive
					// Instead, replace the karavi-authorization-config volume mount with the driver secret volume mount.
					container.VolumeMounts[i] = acorev1.VolumeMountApplyConfiguration{
						Name: &driverSecretNamePlaceholder,
					}
					break
				}
			}
		}
	}

	supportedDriverParams := AuthorizationSupportedDrivers[string(cr.Spec.Driver.CSIDriverType)]
	for i, c := range container.VolumeMounts {
		switch *c.Name {
		case DefaultDriverConfigParamsVolumeMount:
			newName := supportedDriverParams.DriverConfigParamsVolumeMount
			container.VolumeMounts[i].Name = &newName
		case DefaultDriverConfigVolumeMount:
			newConfigName := supportedDriverParams.DriverConfigVolumeMount
			container.VolumeMounts[i].Name = &newConfigName
			newConfigPath := "/" + newConfigName
			container.VolumeMounts[i].MountPath = &newConfigPath
		}
	}

	matched, err := resolveVersionFromConfigMapAuth(ctx, ctrlClient, &cr)
	if err != nil {
		log.Errorw("Image resolution via ConfigMap csm-images failed", "err", err, "specVersion", cr.Spec.Version)
	}
	// Resolve image using the standard precedence: ConfigMap → Custom Registry → default.
	// An independent flag ensures that a sparse ConfigMap (matching version
	// but missing the proxy key) still falls through to Custom Registry.
	matchedImageApplied := false
	if matched.Version != "" {
		proxyKey := "karavi-authorization-proxy"
		if img := matched.Images[proxyKey]; img != "" {
			container.Image = &img
			matchedImageApplied = true
			log.Infow("Overriding container image from ConfigMap csm-images", "key", proxyKey, "image", img, "specVersion", matched.Version)
		}
	}
	if !matchedImageApplied {
		if envImg, found := operatorutils.GetRelatedImage("karavi-authorization-proxy"); found && operatorutils.ShouldUseEnvVarImages(cr, op.CSMVersion) {
			if cr.Spec.CustomRegistry != "" {
				img := operatorutils.ResolveImage(ctx, envImg, cr)
				container.Image = &img
			} else {
				container.Image = &envImg
			}
		} else if cr.Spec.CustomRegistry != "" {
			img := operatorutils.ResolveImage(ctx, *container.Image, cr)
			container.Image = &img
		}
	}

	return &authModule, &container, matched, nil
}

func getAuthApplyVolumes(ctx context.Context, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, auth csmv1.ContainerTemplate, ctrlClient crclient.Client) ([]acorev1.VolumeApplyConfiguration, error) {
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
		version, err := operatorutils.GetVersion(ctx, &cr, op)
		if err != nil {
			return nil, err
		}
		authConfigVersion, err = operatorutils.GetModuleDefaultVersion(version, cr.Spec.Driver.CSIDriverType, csmv1.Authorization, op.ConfigDirectory)
		if err != nil {
			return nil, err
		}
	}

	configMapPath := fmt.Sprintf("%s/moduleconfig/authorization/%s/volumes.yaml", op.ConfigDirectory, authConfigVersion)
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
				vols = vols[:len(vols)-1]
				break
			}
		}
	}

	// Karavi authorization config is not required in config v2.4.0 and later (CSM 1.16) due to driver secret
	driverSecretVersion, err := operatorutils.MinVersionCheck("v2.4.0", authConfigVersion)
	if err != nil {
		return nil, err
	}
	if driverSecretVersion {
		// Do not try to make the karavi-authorization-config volume available if the customer is using the driver secret.
		_, err := operatorutils.GetSecret(ctx, KaraviAuthorizationConfigSecret, cr.GetNamespace(), ctrlClient)
		if err != nil {
			for i, c := range vols {
				if *c.Name == KaraviAuthorizationConfigSecret {
					vols[i] = vols[len(vols)-1]
					vols = vols[:len(vols)-1]
					break
				}
			}
		}
	}

	return vols, nil
}

// AuthInjectDaemonset  - inject authorization into daemonset
func AuthInjectDaemonset(ctx context.Context, ds applyv1.DaemonSetApplyConfiguration, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, ctrlClient crclient.Client) (*applyv1.DaemonSetApplyConfiguration, error) {
	authModule, containerPtr, matched, err := getAuthApplyCR(ctx, cr, op, ctrlClient)
	if err != nil {
		return nil, err
	}

	container := *containerPtr
	operatorutils.UpdateSideCarApply(ctx, authModule.Components, &container, cr, matched, op.CSMVersion)
	vols, err := getAuthApplyVolumes(ctx, cr, op, authModule.Components[0], ctrlClient)
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
func AuthInjectDeployment(ctx context.Context, dp applyv1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule, op operatorutils.OperatorConfig, ctrlClient crclient.Client) (*applyv1.DeploymentApplyConfiguration, error) {
	authModule, containerPtr, matched, err := getAuthApplyCR(ctx, cr, op, ctrlClient)
	if err != nil {
		return nil, err
	}

	container := *containerPtr
	operatorutils.UpdateSideCarApply(ctx, authModule.Components, &container, cr, matched, op.CSMVersion)

	vols, err := getAuthApplyVolumes(ctx, cr, op, authModule.Components[0], ctrlClient)
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

	secrets := []string{"proxy-authz-tokens"}

	authConfigVersion := ""
	if auth.ConfigVersion == "" {
		version, err := operatorutils.GetVersion(ctx, &cr, op)
		if err != nil {
			return err
		}
		authConfigVersion, err = operatorutils.GetModuleDefaultVersion(version, cr.Spec.Driver.CSIDriverType, csmv1.Authorization, op.ConfigDirectory)
		if err != nil {
			return err
		}
	} else {
		authConfigVersion = auth.ConfigVersion
	}

	// Karavi authorization config is not required in config v2.4.0 and later (CSM 1.16) due to driver secret
	driverSecretVersion, err := operatorutils.MinVersionCheck("v2.4.0", authConfigVersion)
	if err != nil {
		return err
	}

	if !driverSecretVersion {
		secrets = append(secrets, KaraviAuthorizationConfigSecret)
	}

	if !skipCertValid {
		secrets = append(secrets, "proxy-server-root-certificate")
	}

	for _, name := range secrets {
		_, err := operatorutils.GetSecret(ctx, name, cr.GetNamespace(), ctrlClient)
		if err != nil {
			log.Error(err, "Failed to query for secret. Warning - the controller pod may not start")
			return fmt.Errorf("failed to find secret %s and certificate validation is requested", name)
		}
	}

	log.Infof("preformed pre-checks for %s", auth.Name)
	return nil
}

// AuthorizationServerPrecheck  - runs precheck for CSM Authorization Proxy Server
func AuthorizationServerPrecheck(ctx context.Context, op operatorutils.OperatorConfig, auth csmv1.Module, cr csmv1.ContainerStorageModule, r operatorutils.ReconcileCSM) error {
	log := logger.GetLogger(ctx)

	authVersion, err := operatorutils.GetVersion(ctx, &cr, op)
	if err != nil {
		return err
	}

	// TODO: Add check for spec.version must be set for CSM 1.16.0 (Authorization 2.4.0) and later

	// Validate the (non-empty) version here
	if err := checkVersion(string(csmv1.Authorization), authVersion, op.ConfigDirectory); err != nil {
		return err
	}

	// Validate gateway vs proxyServerIngress usage based on version and nginx-gateway-fabric component
	isV25OrLater, err := operatorutils.MinVersionCheck("v2.5.0", authVersion)
	if err != nil {
		return fmt.Errorf("error checking authorization version: %v", err)
	}

	nginxGatewayEnabled := false
	for _, component := range auth.Components {
		if component.Name == AuthGatewayComponent && component.Enabled != nil && *component.Enabled {
			nginxGatewayEnabled = true
			break
		}
	}

	// nginx-gateway-fabric component requires v2.5.0+
	if !isV25OrLater && nginxGatewayEnabled {
		return fmt.Errorf("nginx-gateway-fabric component is not supported with authorization v2.4.0 and below; use nginx component instead")
	}

	for _, component := range auth.Components {
		if component.Name == AuthProxyServerComponent {
			if isV25OrLater && nginxGatewayEnabled {
				if len(component.ProxyServerIngress) > 0 {
					return fmt.Errorf("proxyServerIngress is not supported when nginx-gateway-fabric is enabled (v2.5.0+); use the gateway field instead")
				}
			}
			if !isV25OrLater && !nginxGatewayEnabled {
				if component.Gateway != nil {
					return fmt.Errorf("gateway field is not supported with authorization v2.4.0 and below; use proxyServerIngress instead")
				}
			}
		}
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
func getAuthorizationServerDeployment(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, auth csmv1.Module, matched operatorutils.VersionSpec) (string, error) {
	YamlString := ""
	buf, err := readConfigFile(ctx, auth, cr, op, AuthDeploymentManifest)
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	authNamespace := cr.Namespace

	for _, component := range auth.Components {
		// proxy-server component
		if component.Name == AuthProxyServerComponent {
			// Use version-specific default images
			versionDefaults, err := getVersionSpecificDefaultImages(auth.ConfigVersion, op.ConfigDirectory)
			if err != nil {
				return YamlString, fmt.Errorf("failed to get version-specific default images: %w", err)
			}
			defaultRoleImage := DefaultRoleServiceImage
			defaultControllerImage := DefaultControllerImage
			if img, ok := versionDefaults["role-service"]; ok {
				defaultRoleImage = img
			}
			if img, ok := versionDefaults["authorization-controller"]; ok {
				defaultControllerImage = img
			}

			roleServiceImage := getDefaultAuthImage(component.RoleService, defaultRoleImage, matched)
			controllerImage := getDefaultAuthImage(component.AuthorizationController, defaultControllerImage, matched)
			authProxyImages := map[string]*string{
				"role-service":             &roleServiceImage,
				"authorization-controller": &controllerImage,
			}

			for key := range authProxyImages {
				*authProxyImages[key] = getImageForKey(ctx, key, *authProxyImages[key], cr, matched, op.CSMVersion)
			}

			YamlString = strings.ReplaceAll(YamlString, AuthRoleServiceImage, *authProxyImages["role-service"])
			YamlString = strings.ReplaceAll(YamlString, AuthRoleServiceReplicas, strconv.Itoa(component.RoleServiceReplicas))
			YamlString = strings.ReplaceAll(YamlString, AuthControllerImage, *authProxyImages["authorization-controller"])
			YamlString = strings.ReplaceAll(YamlString, AuthControllerReplicas, strconv.Itoa(component.AuthorizationControllerReplicas))
			YamlString = strings.ReplaceAll(YamlString, AuthLeaderElectionEnabled, strconv.FormatBool(component.LeaderElection))
			YamlString = strings.ReplaceAll(YamlString, AuthControllerReconcileInterval, component.ControllerReconcileInterval)
			YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)
			YamlString = strings.ReplaceAll(YamlString, AuthCSMNameSpace, cr.Namespace)
		}

		// redis component
		if component.Name == AuthRedisComponent {
			YamlString = strings.ReplaceAll(YamlString, AuthRedisName, component.RedisName)
			YamlString = strings.ReplaceAll(YamlString, AuthRedisCommander, component.RedisCommander)
			YamlString = strings.ReplaceAll(YamlString, AuthRedisSentinel, component.Sentinel)
			YamlString = strings.ReplaceAll(YamlString, AuthCSMNameSpace, cr.Namespace)

			ok, err := validateRedisConfig(component)
			if err != nil {
				return YamlString, fmt.Errorf("validating redis: %w", err)
			}

			// Create redis kubernetes secret if credentials are provided
			// This path is used when no secret provider class is configured
			if ok {
				redisSecret := createRedisK8sSecret(defaultRedisSecretName, cr.Namespace, component.RedisUsername, component.RedisPassword)
				secretYaml, err := yaml.Marshal(redisSecret)
				if err != nil {
					return YamlString, fmt.Errorf("failed to marshal redis kubernetes secret: %w", err)
				}

				YamlString += fmt.Sprintf("\n---\n%s", secretYaml)
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, AuthNamespace, authNamespace)
	YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)
	YamlString = strings.ReplaceAll(YamlString, AuthCSMNameSpace, cr.Namespace)

	return YamlString, nil
}

// AuthorizationServerDeployment - apply/delete deployment objects
func AuthorizationServerDeployment(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client, matched operatorutils.VersionSpec) error {
	log := logger.GetLogger(ctx)

	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	YamlString, err := getAuthorizationServerDeployment(ctx, op, cr, authModule, matched)
	if err != nil {
		return err
	}

	err = applyDeleteObjects(ctx, ctrlClient, YamlString, isDeleting)
	if err != nil {
		return err
	}

	log.Infof("Config version of auth module: %s", authModule.ConfigVersion)
	// scaffolds are applied only for v2.3.0 and above for secret provider class mounts and volumes
	ok, err := operatorutils.MinVersionCheck("v2.3.0", authModule.ConfigVersion)
	if err != nil {
		return err
	}

	if ok {
		err = applyDeleteAuthorizationRedisStatefulsetV2(ctx, isDeleting, cr, ctrlClient, authModule, matched, op.CSMVersion)
		if err != nil {
			return err
		}

		err = applyDeleteAuthorizationRediscommanderDeploymentV2(ctx, isDeleting, cr, ctrlClient, authModule, matched, op.CSMVersion)
		if err != nil {
			return err
		}

		err = applyDeleteAuthorizationSentinelStatefulsetV2(ctx, isDeleting, cr, ctrlClient, authModule, matched, op.CSMVersion)
		if err != nil {
			return err
		}

		err = applyDeleteAuthorizationProxyServerV2(ctx, isDeleting, cr, ctrlClient, authModule, matched, op)
		if err != nil {
			return err
		}

		err = applyDeleteAuthorizationTenantServiceV2(ctx, isDeleting, cr, ctrlClient, authModule, matched, op)
		if err != nil {
			return err
		}
	}

	err = authorizationStorageServiceV2(ctx, isDeleting, cr, ctrlClient, authModule, matched, op)
	if err != nil {
		return err
	}

	return nil
}

func authorizationStorageServiceV2(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client, authModule csmv1.Module, matched operatorutils.VersionSpec, op operatorutils.OperatorConfig) error {
	log := logger.GetLogger(ctx)
	// SecretProviderClasses and K8s secret for storage credentials is supported from config v2.3.0 (CSM 1.15) onwards
	storageCreds, err := operatorutils.MinVersionCheck("v2.3.0", authModule.ConfigVersion)
	if err != nil {
		return err
	}

	replicas := 0
	sentinelName := ""
	redisReplicas := 0
	image := ""
	var secretProviderClasses *csmv1.StorageSystemSecretProviderClasses
	var secrets []string
	leaderElection := true
	otelCollector := ""
	configSecretName = defaultConfigSecretName
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthProxyServerComponent:
			replicas = component.StorageServiceReplicas
			// Use version-specific default image
			versionDefaults, err := getVersionSpecificDefaultImages(authModule.ConfigVersion, op.ConfigDirectory)
			if err != nil {
				return fmt.Errorf("failed to get version-specific default images: %w", err)
			}
			defaultStorageImage := DefaultStorageServiceImage
			if img, ok := versionDefaults["storage-service"]; ok {
				defaultStorageImage = img
			}
			image = getDefaultAuthImage(component.StorageService, defaultStorageImage, matched)
			image = getImageForKey(ctx, "storage-service", image, cr, matched, op.CSMVersion)
			leaderElection = component.LeaderElection
			otelCollector = component.OpenTelemetryCollectorAddress
		case AuthRedisComponent:
			sentinelName = component.Sentinel
			redisReplicas = component.RedisReplicas
			updateRedisGlobalVars(component)
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
	err := ctrlClient.Get(ctx, crclient.ObjectKey{
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

func applyDeleteAuthorizationProxyServerV2(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client, authModule csmv1.Module, matched operatorutils.VersionSpec, op operatorutils.OperatorConfig) error {
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
			// Use version-specific default image
			versionDefaults, err := getVersionSpecificDefaultImages(authModule.ConfigVersion, op.ConfigDirectory)
			if err != nil {
				return fmt.Errorf("failed to get version-specific default images: %w", err)
			}
			defaultProxyImage := DefaultProxyServerImage
			if img, ok := versionDefaults["proxy-service"]; ok {
				defaultProxyImage = img
			}
			proxyImage = getDefaultAuthImage(component.ProxyService, defaultProxyImage, matched)
			proxyImage = getImageForKey(ctx, "proxy-service", proxyImage, cr, matched, op.CSMVersion)
			// OPA images are not version-specific, use defaults
			opaImage = getDefaultAuthImage(component.Opa, DefaultOpaImage, matched)
			opaImage = getImageForKey(ctx, "opa", opaImage, cr, matched, op.CSMVersion)
			opaKubeMgmtImage = getDefaultAuthImage(component.OpaKubeMgmt, DefaultOpaKubeMgmtImage, matched)
			opaKubeMgmtImage = getImageForKey(ctx, "opa-kube-mgmt", opaKubeMgmtImage, cr, matched, op.CSMVersion)
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

func applyDeleteAuthorizationTenantServiceV2(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client, authModule csmv1.Module, matched operatorutils.VersionSpec, op operatorutils.OperatorConfig) error {
	replicas := 0
	redisReplicas := 0
	image := ""
	sentinelName := ""
	configSecretName = defaultConfigSecretName
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthProxyServerComponent:
			// Use version-specific default image
			versionDefaults, err := getVersionSpecificDefaultImages(authModule.ConfigVersion, op.ConfigDirectory)
			if err != nil {
				return fmt.Errorf("failed to get version-specific default images: %w", err)
			}
			defaultTenantImage := DefaultTenantServiceImage
			if img, ok := versionDefaults["tenant-service"]; ok {
				defaultTenantImage = img
			}
			image = getDefaultAuthImage(component.TenantService, defaultTenantImage, matched)
			image = getImageForKey(ctx, "tenant-service", image, cr, matched, op.CSMVersion)
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

func applyDeleteAuthorizationRedisStatefulsetV2(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client, authModule csmv1.Module, matched operatorutils.VersionSpec, operatorCSMVersion string) error {
	redisName := ""
	image := ""
	redisReplicas := 0

	for _, component := range authModule.Components {
		switch component.Name {
		case AuthRedisComponent:
			redisName = component.RedisName
			image = getDefaultAuthImage(component.Redis, DefaultRedisImage, matched)
			image = getImageForKey(ctx, "redis", image, cr, matched, operatorCSMVersion)
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

func applyDeleteAuthorizationRediscommanderDeploymentV2(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client, authModule csmv1.Module, matched operatorutils.VersionSpec, operatorCSMVersion string) error {
	rediscommanderName := ""
	sentinelName := ""
	image := ""
	redisReplicas := 0
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthRedisComponent:
			rediscommanderName = component.RedisCommander
			sentinelName = component.Sentinel
			image = getDefaultAuthImage(component.Commander, DefaultRedisCommanderImage, matched)
			image = getImageForKey(ctx, "commander", image, cr, matched, operatorCSMVersion)
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

func applyDeleteAuthorizationSentinelStatefulsetV2(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client, authModule csmv1.Module, matched operatorutils.VersionSpec, operatorCSMVersion string) error {
	sentinelName := ""
	redisName := ""
	image := ""
	redisReplicas := 0
	for _, component := range authModule.Components {
		switch component.Name {
		case AuthRedisComponent:
			sentinelName = component.Sentinel
			redisName = component.RedisName
			image = getDefaultAuthImage(component.Redis, DefaultRedisImage, matched)
			image = getImageForKey(ctx, "redis", image, cr, matched, operatorCSMVersion)
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

// AuthorizationIngress - apply/delete ingress or HTTPRoute objects depending on the module version.
// For v2.5.0 and later, creates an HTTPRoute (Gateway API). For v2.4.0 and below, creates an Ingress.
func AuthorizationIngress(ctx context.Context, isDeleting, isOpenShift bool, cr csmv1.ContainerStorageModule, r operatorutils.ReconcileCSM, ctrlClient crclient.Client) error {
	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return err
	}

	isV25OrLater, err := operatorutils.MinVersionCheck("v2.5.0", auth.ConfigVersion)
	if err != nil {
		return fmt.Errorf("error checking authorization version: %v", err)
	}

	if isV25OrLater && !isOpenShift {
		return authorizationHTTPRoute(ctx, isDeleting, cr, r, ctrlClient)
	}

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

// authorizationHTTPRoute - apply/delete Gateway API HTTPRoute for authorization proxy server (v2.5.0+)
func authorizationHTTPRoute(ctx context.Context, isDeleting bool, cr csmv1.ContainerStorageModule, r operatorutils.ReconcileCSM, ctrlClient crclient.Client) error {
	route, err := createHTTPRoute(cr)
	if err != nil {
		return fmt.Errorf("creating HTTPRoute: %v", err)
	}

	routeBytes, err := json.Marshal(route)
	if err != nil {
		return fmt.Errorf("marshaling HTTPRoute: %v", err)
	}

	routeYaml, err := yaml.JSONToYAML(routeBytes)
	if err != nil {
		return fmt.Errorf("converting HTTPRoute to YAML: %v", err)
	}

	// Wait for Gateway API controller to be ready before creating HTTPRoutes
	if !isDeleting {
		if err := operatorutils.WaitForGatewayController(ctx, cr, r, time.Duration(10)*time.Second); err != nil {
			return fmt.Errorf("Gateway API controller is not ready: %v", err)
		}
	}

	return applyDeleteObjects(ctx, ctrlClient, string(routeYaml), isDeleting)
}

// getGatewayController - configure Gateway API controller yaml with the namespace before installation (v2.5.0+)
func getGatewayController(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return YamlString, err
	}

	buf, err := readConfigFile(ctx, auth, cr, op, AuthGatewayManifest)
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

// GatewayController - apply/delete Gateway API controller objects (v2.5.0+)
func GatewayController(ctx context.Context, isDeleting bool, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	YamlString, err := getGatewayController(ctx, op, cr)
	if err != nil {
		return err
	}

	err = applyDeleteObjects(ctx, ctrlClient, YamlString, isDeleting)
	if err != nil {
		return err
	}

	// Clean up data plane resources created by the nginx-gateway-fabric controller.
	// These are not part of the operator manifest but are spawned by the controller
	// when it processes the Gateway resource. On deletion the controller deployment
	// is removed first, so it can no longer clean up its own child resources.
	if isDeleting {
		ns := cr.GetNamespace()
		dataPlane := ns + "-gateway-nginx"

		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: dataPlane, Namespace: ns},
		}
		deploy.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
		if err := operatorutils.DeleteObject(ctx, deploy, ctrlClient); err != nil {
			return fmt.Errorf("deleting gateway data plane deployment: %w", err)
		}

		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: dataPlane, Namespace: ns},
		}
		svc.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Service"))
		if err := operatorutils.DeleteObject(ctx, svc, ctrlClient); err != nil {
			return fmt.Errorf("deleting gateway data plane service: %w", err)
		}
	}

	return nil
}

// getNginxIngressController - configure nginx ingress controller with the specified namespace before installation
func getNginxIngressController(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return YamlString, err
	}

	buf, err := readConfigFile(ctx, auth, cr, op, AuthNginxIngressManifest)
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
	YamlString, err := getNginxIngressController(ctx, op, cr)
	if err != nil {
		return err
	}

	err = applyDeleteObjects(ctx, ctrlClient, YamlString, isDeleting)
	if err != nil {
		return err
	}

	return nil
}

// NginxIngressControllerCleanup - delete old NGINX ingress controller objects during upgrade to Gateway API
func NginxIngressControllerCleanup(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	// For v2.5.0+ upgrades, we need to use older version NGINX config to clean up old resources
	// Support n-2 upgrades: try v2.4.0, v2.3.0, v2.2.0
	previousVersions := []string{"v2.4.0", "v2.3.0", "v2.2.0"}

	_, err := getAuthorizationModule(cr)
	if err != nil {
		// If authorization module not found, nothing to cleanup
		return nil
	}

	// Try each previous version until we find one with NGINX config
	for _, version := range previousVersions {
		tempCR := cr.DeepCopy()
		for i := range tempCR.Spec.Modules {
			if tempCR.Spec.Modules[i].Name == csmv1.AuthorizationServer {
				tempCR.Spec.Modules[i].ConfigVersion = version
				break
			}
		}

		YamlString, err := getNginxIngressController(ctx, op, *tempCR)
		if err == nil {
			// Found NGINX config, delete the resources
			return applyDeleteObjects(ctx, ctrlClient, YamlString, true)
		}
	}

	// No NGINX config found in any previous version, nothing to cleanup
	return nil
}

// getPolicies - configure policies with the specified namespace before installation
func getPolicies(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return YamlString, err
	}

	buf, err := readConfigFile(ctx, auth, cr, op, AuthPolicyManifest)
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
	YamlString, err := getPolicies(ctx, op, cr)
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
			buf, err := readConfigFile(ctx, authModule, cr, op, AuthCustomCert)
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
func getAuthCrdDeploy(ctx context.Context, op operatorutils.OperatorConfig, cr csmv1.ContainerStorageModule, auth csmv1.Module) (string, error) {
	yamlString := ""
	buf, err := readConfigFile(ctx, auth, cr, op, AuthCrds)
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
	if ok, err := operatorutils.MinVersionCheck("v2.0.0-alpha", auth.ConfigVersion); !ok {
		return nil
	} else if err != nil {
		return err
	}

	yamlString, err := getAuthCrdDeploy(ctx, op, cr, auth)
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

// createHTTPRoute builds a Gateway API HTTPRoute for the proxy-server (v2.5.0+)
func createHTTPRoute(cr csmv1.ContainerStorageModule) (*gatewayv1.HTTPRoute, error) {
	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return nil, err
	}

	var hosts []gatewayv1.Hostname
	gatewayName := cr.Namespace + "-gateway"
	gwNamespace := gatewayv1.Namespace(cr.Namespace)
	annotations := make(map[string]string)

	for _, component := range authModule.Components {
		if component.Name == AuthProxyServerComponent {
			if component.Hostname != "" {
				hosts = append(hosts, gatewayv1.Hostname(component.Hostname))
			}
			for _, proxyIngress := range component.ProxyServerIngress {
				for _, host := range proxyIngress.Hosts {
					hosts = append(hosts, gatewayv1.Hostname(host))
				}
				for k, v := range proxyIngress.Annotations {
					annotations[k] = v
				}
			}
			if component.Gateway != nil {
				for _, host := range component.Gateway.Hosts {
					hosts = append(hosts, gatewayv1.Hostname(host))
				}
				for k, v := range component.Gateway.Annotations {
					annotations[k] = v
				}
			}
		}
	}

	pathType := gatewayv1.PathMatchPathPrefix
	pathValue := "/"
	port := gatewayv1.PortNumber(8080)

	httpRoute := &gatewayv1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "gateway.networking.k8s.io/v1",
			Kind:       "HTTPRoute",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "proxy-server",
			Namespace:   cr.Namespace,
			Annotations: annotations,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Name:      gatewayv1.ObjectName(gatewayName),
						Namespace: &gwNamespace,
					},
				},
			},
			Hostnames: hosts,
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &pathType,
								Value: &pathValue,
							},
						},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: "proxy-server",
									Port: &port,
								},
							},
						},
					},
				},
			},
		},
	}

	return httpRoute, nil
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

			// gateway.hosts (v2.5.0+)
			if component.Gateway != nil {
				hosts = append(hosts, component.Gateway.Hosts...)
			}
		}
	}

	return hosts, nil
}

func getClassName(isOpenShift bool, cr csmv1.ContainerStorageModule) (string, error) {
	if isOpenShift {
		return "openshift-default", nil
	}

	authModule, err := getAuthorizationModule(cr)
	if err != nil {
		return "", err
	}

	for _, component := range authModule.Components {
		if component.Name == AuthProxyServerComponent {
			for _, proxyServerIngress := range component.ProxyServerIngress {
				proxyIngressClassName = proxyServerIngress.IngressClassName
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

// validateRedisConfig - validate the redis parameters
// returns true if direct credentials are provided, false if secret provider class is used
func validateRedisConfig(component csmv1.ContainerTemplate) (bool, error) {
	hasUsername := component.RedisUsername != ""
	hasPassword := component.RedisPassword != ""
	hasDirectCredentials := hasUsername && hasPassword

	// Check that both username and password are provided together (if either is provided)
	if hasUsername != hasPassword {
		return false, fmt.Errorf("redisUsername and redisPassword must be provided together")
	}

	// Direct credentials not provided, check for secret provider class
	hasSecretProviderClass := len(component.RedisSecretProviderClass) > 0
	validSecretProviderClass := false
	for _, config := range component.RedisSecretProviderClass {
		// Skip empty entries (all fields empty)
		if config.SecretProviderClassName == "" && config.RedisSecretName == "" && config.RedisUsernameKey == "" && config.RedisPasswordKey == "" {
			continue
		}

		// If any field is provided, all must be provided
		if config.SecretProviderClassName == "" || config.RedisSecretName == "" || config.RedisUsernameKey == "" || config.RedisPasswordKey == "" {
			return false, fmt.Errorf("redisSecretProviderClass requires all of: secretProviderClassName, redisSecretName, redisUsernameKey, and redisPasswordKey")
		}
		validSecretProviderClass = true
	}

	// Check for conflicting configurations
	if hasDirectCredentials && validSecretProviderClass {
		return false, fmt.Errorf("specify either redisUsername/redisPassword or redisSecretProviderClass, not both")
	}

	// Check that at least one method is provided
	if !hasDirectCredentials && !validSecretProviderClass {
		if hasSecretProviderClass {
			return false, fmt.Errorf("redisSecretProviderClass is incomplete. All of the following must be specified: secretProviderClassName, redisSecretName, redisUsernameKey, and redisPasswordKey")
		}
		return false, fmt.Errorf("redis credentials are required. Either set redisUsername and redisPassword or configure redisSecretProviderClass to use a Secret Store CSI driver")
	}

	return hasDirectCredentials, nil
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

func getImageForKey(ctx context.Context, key string, defaultImage string, cr csmv1.ContainerStorageModule, matched operatorutils.VersionSpec, operatorCSMVersion string) string {
	// Config map gets highest priority
	returnImg := defaultImage
	matchedImageApplied := false
	if matched.Version != "" {
		if img := matched.Images[key]; img != "" {
			returnImg = img
			matchedImageApplied = true
		}
	}
	if !matchedImageApplied {
		if envImg, found := operatorutils.GetRelatedImage(key); found && operatorutils.ShouldUseEnvVarImages(cr, operatorCSMVersion) {
			// Environment variable takes next priority
			if cr.Spec.CustomRegistry != "" {
				returnImg = operatorutils.ResolveImage(ctx, envImg, cr)
			} else {
				returnImg = envImg
			}
		} else if cr.Spec.CustomRegistry != "" {
			// Followed by custom registry
			returnImg = operatorutils.ResolveImage(ctx, returnImg, cr)
		}
	}

	return returnImg
}

// getDefaultAuthImage returns the final images for the Auth component
// If the CSM version is specified, default image for the ConfigMap is returned
// Else the default image from the Auth component is returned
func getDefaultAuthImage(componentImage, defaultImage string, _ operatorutils.VersionSpec) string {
	if componentImage == "" {
		return defaultImage
	}
	return componentImage
}

// getLatestAuthVersion returns the latest authorization version by scanning operatorconfig
func getLatestAuthVersion(configDirectory string) (string, error) {
	authConfigPath := fmt.Sprintf("%s/moduleconfig/%s", configDirectory, csmv1.Authorization)

	// Read the directory to find all version subdirectories
	files, err := os.ReadDir(authConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to read authorization config directory %s: %w", authConfigPath, err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no authorization versions found in directory %s", authConfigPath)
	}

	latestVersion := "v0.0.0"
	for _, file := range files {
		if file.IsDir() {
			version := file.Name()
			// Simple version comparison - assumes semantic versioning
			if version > latestVersion {
				latestVersion = version
			}
		}
	}

	if latestVersion == "v0.0.0" {
		return "", fmt.Errorf("no valid authorization versions found in directory %s", authConfigPath)
	}

	return latestVersion, nil
}

// getVersionSpecificDefaultImages returns the default images for a given config version
// It dynamically constructs image tags based on the config version
func getVersionSpecificDefaultImages(configVersion string, configDirectory string) (map[string]string, error) {
	images := make(map[string]string)

	// Construct version-specific images by appending the version tag
	tag := configVersion
	if tag == "" {
		// Infer latest version from operatorconfig directory instead of hardcoding
		latestVersion, err := getLatestAuthVersion(configDirectory)
		if err != nil {
			return nil, fmt.Errorf("failed to determine latest authorization version: %w", err)
		}
		tag = latestVersion
	}

	images["proxy-service"] = fmt.Sprintf("%s:%s", DefaultProxyServerImage, tag)
	images["tenant-service"] = fmt.Sprintf("%s:%s", DefaultTenantServiceImage, tag)
	images["role-service"] = fmt.Sprintf("%s:%s", DefaultRoleServiceImage, tag)
	images["storage-service"] = fmt.Sprintf("%s:%s", DefaultStorageServiceImage, tag)
	images["authorization-controller"] = fmt.Sprintf("%s:%s", DefaultControllerImage, tag)

	return images, nil
}
