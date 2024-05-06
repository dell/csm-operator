//  Copyright © 2021 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	csmv1 "github.com/dell/csm-operator/api/v1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/logger"
	utils "github.com/dell/csm-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// TODO (SHAYNA): test with custom certs
// TODO (SHAYNA): test on ocp, check for valid ingress/annotations
// TODO (SHAYNA): test on k8s, check for valid ingress/annotations
// TODO (SHAYNA): test with mulitple annotations
// TODO (SHAYNA): test with mulitple hosts


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
	// AuthSelfSignedCert - self-signed certificate file
	AuthSelfSignedCert = "selfsigned-cert.yaml"
	// AuthCustomCert - custom certificate file
	AuthCustomCert = "custom-cert.yaml"

	// AuthNamespace -
	AuthNamespace = "<NAMESPACE>"
	// AuthServerImage -
	AuthServerImage = "<AUTHORIZATION_PROXY_SERVER_IMAGE>"
	// AuthOpaImage -
	AuthOpaImage = "<AUTHORIZATION_OPA_IMAGE>"
	// AuthOpaKubeMgmtImage -
	AuthOpaKubeMgmtImage = "<AUTHORIZATION_OPA_KUBEMGMT_IMAGE>"
	// AuthTenantServiceImage -
	AuthTenantServiceImage = "<AUTHORIZATION_TENANT_SERVICE_IMAGE>"
	// AuthRoleServiceImage -
	AuthRoleServiceImage = "<AUTHORIZATION_ROLE_SERVICE_IMAGE>"
	// AuthStorageServiceImage -
	AuthStorageServiceImage = "<AUTHORIZATION_STORAGE_SERVICE_IMAGE>"
	// AuthRedisImage -
	AuthRedisImage = "<AUTHORIZATION_REDIS_IMAGE>"
	// AuthRedisCommanderImage -
	AuthRedisCommanderImage = "<AUTHORIZATION_REDIS_COMMANDER_IMAGE>"
	// AuthRedisStorageClass -
	AuthRedisStorageClass = "<REDIS_STORAGE_CLASS>"

	// AuthProxyHost -
	AuthProxyHost = "<AUTHORIZATION_HOSTNAME>"
	// AuthProxyIngressHost -
	AuthProxyIngressHost = "<PROXY_INGRESS_HOST>"
	// AuthProxyIngressClassName -
	AuthProxyIngressClassName = "<PROXY_INGRESS_CLASSNAME>"

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

	// AuthLocalStorageClass -
	AuthLocalStorageClass = "csm-authorization-local-storage"
)

var (
	redisStorageClass     string
	authHostname          string
	proxyIngressHosts     []string
	proxyIngressClassName string
	authCertificate       string
	authPrivateKey        string
)

type Ingress struct {
	IngressClassName string
	Annotations      map[string]string
	Hosts            []string
}

// AuthorizationSupportedDrivers is a map containing the CSI Drivers supported by CSM Authorization. The key is driver name and the value is the driver plugin identifier
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
	proxyServerSecrets := []string{"karavi-config-secret", "karavi-storage-secret", "karavi-auth-tls"}
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
			YamlString = strings.ReplaceAll(YamlString, AuthOpaImage, component.Opa)
			YamlString = strings.ReplaceAll(YamlString, AuthOpaKubeMgmtImage, component.OpaKubeMgmt)
			YamlString = strings.ReplaceAll(YamlString, AuthTenantServiceImage, component.TenantService)
			YamlString = strings.ReplaceAll(YamlString, AuthRoleServiceImage, component.RoleService)
			YamlString = strings.ReplaceAll(YamlString, AuthStorageServiceImage, component.StorageService)
			YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)
		}

		// redis component
		if component.Name == AuthRedisComponent {
			YamlString = strings.ReplaceAll(YamlString, AuthRedisImage, component.Redis)
			YamlString = strings.ReplaceAll(YamlString, AuthRedisCommanderImage, component.Commander)

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
		deployObjects, err := utils.GetModuleComponentObj([]byte(yamlString))
		if err != nil {
			return err
		}

		for _, ctrlObj := range deployObjects {
			if isDeleting {
				if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
					return err
				}
			} else {
				if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
					return err
				}
			}
		}
	}

	YamlString, err := getAuthorizationServerDeployment(op, cr)
	if err != nil {
		return err
	}
	deployObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range deployObjects {
		if isDeleting {
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	return nil
}

// getAuthorizationIngressRules - apply dynamic values to the Ingress manifest before installation
func getAuthorizationIngressRules(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return YamlString, err
	}

	// get annotations
	annotations, err := addAnnotationsToIngress(cr)
	if err != nil {
		return YamlString, fmt.Errorf("adding annotations to ingress: %v", err)
	}

	// get hosts
	hosts, err := addHostsToIngress(cr)
	if err != nil {
		return YamlString, fmt.Errorf("adding hosts to ingress: %v", err)
	}

	// set the ingress class name in the Ingress manifest on kubernetes
	for _, m := range cr.Spec.Modules {
		if !m.OpenShift {
			i := Ingress{
				IngressClassName: AuthProxyIngressClassName,
			}

			yamlData, err := yaml.Marshal(&i)
			if err != nil {
				return "", fmt.Errorf("error while marshaling ingress class name %v", err)
			}

			err = os.WriteFile(AuthIngressManifest, yamlData, 0644)
			if err != nil {
				return "", fmt.Errorf("failed writing to %s: %v", AuthIngressManifest, err)
			}
		}
	}

	buf, err := readConfigFile(auth, cr, op, AuthIngressManifest)
	if err != nil {
		return YamlString, err
	}

	YamlString = string(buf)
	authNamespace := cr.Namespace

	for _, component := range auth.Components {
		if component.Name == AuthProxyServerComponent {
			authHostname = component.Hostname

			for _, proxyServerIngress := range component.ProxyServerIngress {
				for _, m := range cr.Spec.Modules {
					if !m.OpenShift {
						proxyIngressClassName = proxyServerIngress.IngressClassName
						YamlString = strings.ReplaceAll(YamlString, AuthProxyIngressClassName, proxyIngressClassName)
					}
				}
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, AuthNamespace, authNamespace)
	YamlString = strings.ReplaceAll(YamlString, "{}", string(annotations))
	YamlString = strings.ReplaceAll(YamlString, AuthProxyHost, authHostname)
	YamlString = strings.ReplaceAll(YamlString, AuthProxyIngressHost, string(hosts))
	YamlString = strings.ReplaceAll(YamlString, CSMName, cr.Name)

	return YamlString, nil
}

// AuthorizationIngress - apply/delete ingress objects
func AuthorizationIngress(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, r utils.ReconcileCSM, ctrlClient crclient.Client) error {
	YamlString, err := getAuthorizationIngressRules(op, cr)
	if err != nil {
		return err
	}
	ingressObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	// Wait for NGINX ingress controller to be ready before creating Ingresses
	if !isDeleting {
		if err := utils.WaitForNginxController(ctx, cr, r, time.Duration(10)*time.Second); err != nil {
			return fmt.Errorf("NGINX ingress controller is not ready: %v", err)
		}
	}

	for _, ctrlObj := range ingressObjects {
		if isDeleting {
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
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

	ctrlObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range ctrlObjects {
		if isDeleting {
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
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

	deployObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range deployObjects {
		if isDeleting {
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	return nil
}

func getCerts(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""
	authNamespace := cr.Namespace

	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return YamlString, err
	}

	// get hosts
	hosts, err := addHostsToIngress(cr)
	if err != nil {
		return YamlString, fmt.Errorf("adding hosts to ingress: %v", err)
	}

	for _, component := range auth.Components {
		if component.Name == AuthProxyServerComponent {
			authHostname = component.Hostname
			authCertificate = component.Certificate
			authPrivateKey = component.PrivateKey
		}
	}

	if authCertificate == "" || authPrivateKey == "" {
		// use custom tls secret
		if authCertificate == "" && authPrivateKey == "" {
			buf, err := readConfigFile(auth, cr, op, AuthCustomCert)
			if err != nil {
				return YamlString, err
			}

			YamlString = string(buf)
			YamlString = strings.ReplaceAll(YamlString, AuthCert, authCertificate)
			YamlString = strings.ReplaceAll(YamlString, AuthPrivateKey, authPrivateKey)
		} else {
			return YamlString, fmt.Errorf("authorization install failed -- either cert or privatekey missing for custom cert")
		}
	} else {
		// using self-signed cert
		buf, err := readConfigFile(auth, cr, op, AuthSelfSignedCert)
		if err != nil {
			return YamlString, err
		}

		YamlString = string(buf)
		YamlString = strings.ReplaceAll(YamlString, AuthProxyHost, authHostname)
		YamlString = strings.ReplaceAll(YamlString, AuthProxyIngressHost, string(hosts))
	}

	YamlString = strings.ReplaceAll(YamlString, AuthNamespace, authNamespace)

	return YamlString, nil
}

func InstallWithCerts(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	YamlString, err := getCerts(op, cr)
	if err != nil {
		return err
	}

	deployObjects, err := utils.GetModuleComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range deployObjects {
		if isDeleting {
			if err := utils.DeleteObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		} else {
			if err := utils.ApplyObject(ctx, ctrlObj, ctrlClient); err != nil {
				return err
			}
		}
	}

	return nil
}

func addAnnotationsToIngress(cr csmv1.ContainerStorageModule) ([]byte, error) {
	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return nil, err
	}

	annotations := make(map[string]string)
	for _, component := range auth.Components {
		if component.Name == AuthProxyServerComponent {
			for _, proxyServerIngress := range component.ProxyServerIngress {
				if len(proxyServerIngress.Annotations) != 0 {
					for k, v := range proxyServerIngress.Annotations {
						annotations[k] = v
					}
				}

				for _, m := range cr.Spec.Modules {
					// set termination route annotation on openshift
					if m.OpenShift {
						annotations["route.openshift.io/termination"] = "edge"
					}
				}
			}
		}
	}

	a := Ingress{
		Annotations: annotations,
	}

	yamlData, err := yaml.Marshal(&a)
	if err != nil {
		return nil, fmt.Errorf("error while marshaling authorization annotations %v", err)
	}

	return yamlData, err
}

func addHostsToIngress(cr csmv1.ContainerStorageModule) ([]byte, error) {
	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return nil, err
	}

	for _, component := range auth.Components {
		if component.Name == AuthProxyServerComponent {
			for _, proxyServerIngress := range component.ProxyServerIngress {
				  // TODO (SHAYNA): if more than 1 host, need to add another host path in ingress
				if len(proxyServerIngress.Hosts) != 0 {
					for _, h := range proxyServerIngress.Hosts {
						proxyIngressHosts = append(proxyServerIngress.Hosts, h)
					}
				}
			}
		}
	}

	h := Ingress{
		Hosts: proxyIngressHosts,
	}

	yamlData, err := yaml.Marshal(&h)
	if err != nil {
		return nil, fmt.Errorf("error while marshaling authorization hosts %v", err)
	}

	return yamlData, err
}
