package modules

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

const (
	// AuthorizationDeploymentManifest - deployment resources and ingress rules for authorization module
	AuthDeploymentManifest = "deployment.yaml"
	// AuthorizationIngressManifest -
	AuthIngressManifest = "ingress.yaml"
	// AuthCertManagerManifest -
	AuthCertManagerManifest = "cert-manager.yaml"
	// AuthNginxIngressManifest -
	AuthNginxIngressManifest = "nginx-ingress-controller.yaml"
	// AuthPolicyManifest -
	AuthPolicyManifest = "policies.yaml"

	// AuthNamespace -
	AuthNamespace = "<NAMESPACE>"
	// AuthLogLevel -
	AuthLogLevel = "<AUTHORIZATION_LOG_LEVEL>"
	// AuthZipkinCollectorUri -
	AuthZipkinCollectorUri = "<AUTHORIZATION_ZIPKIN_COLLECTORURI>"
	// AuthZipkinProbability -
	AuthZipkinProbability = "<AUTHORIZATION_ZIPKIN_PROBABILITY>"
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
	// AuthStorageClass -
	AuthRedisStorageClass = "<REDIS_STORAGE_CLASS>"

	// AuthProxyHost -
	AuthProxyHost = "<AUTHORIZATION_HOSTNAME>"
	// AuthProxyIngressHost -
	AuthProxyIngressHost = "<PROXY_INGRESS_HOSTS>"
	// AuthProxyIngressClassName -
	AuthProxyIngressClassName = "<PROXY_INGRESS_CLASSNAME>"
	// AuthTenantIngressClassName -
	AuthTenantIngressClassName = "<TENANT_INGRESS_CLASSNAME>"
	// AuthRoleIngressClassName -
	AuthRoleIngressClassName = "<ROLE_INGRESS_CLASSNAME>"
	// AuthStorageIngressClassName -
	AuthStorageIngressClassName = "<STORAGE_INGRESS_CLASSNAME>"

	// AuthProxyServerComponent - karavi-authorization-proxy-server component
	AuthProxyServerComponent = "karavi-authorization-proxy-server"
	// AuthSidecarComponent - karavi-authorization-proxy component
	AuthSidecarComponent = "karavi-authorization-proxy"
	// AuthNginxIngressComponent - ingress-nginx component
	AuthNginxIngressComponent = "ingress-nginx"
	// AuthCertManagerComponent - cert-manager component
	AuthCertManagerComponent = "cert-manager"
)

// AuthorizationSupportedDrivers is a map containing the CSI Drivers supported by CSM Authorization. The key is driver name and the value is the driver plugin identifier
var AuthorizationSupportedDrivers = map[string]SupportedDriverParam{
	"powerscale": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	},
	"isilon": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	}, // either powerscale or isilon are valid types
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
			return fmt.Errorf("extpected notation value to be true but got %s", annotation["com.dell.karavi-authorization-proxy"])
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
func CheckApplyContainersAuth(containers []acorev1.ContainerApplyConfiguration, drivertype string) error {
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
				if *env.Name == "SKIP_CERTIFICATE_VALIDATION" {
					if _, err := strconv.ParseBool(*env.Value); err != nil {
						return fmt.Errorf("%s is an invalid value for SKIP_CERTIFICATE_VALIDATION: %v", *env.Value, err)
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
		if env.Name == "SKIP_CERTIFICATE_VALIDATION" {
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
		err := ctrlClient.Get(ctx, types.NamespacedName{Name: name,
			Namespace: cr.GetNamespace()}, found)
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

// getAuthorizationServerDeployment - update deployment manifest
func getAuthorizationServerDeployment(op utils.OperatorConfig, cr csmv1.ContainerStorageModule, auth csmv1.Module) (string, error) {
	YamlString := ""

	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return YamlString, err
	}

	deploymentPath := fmt.Sprintf("%s/moduleconfig/authorization/%s/%s", op.ConfigDirectory, auth.ConfigVersion, AuthDeploymentManifest)
	buf, err := os.ReadFile(filepath.Clean(deploymentPath))
	if err != nil {
		return YamlString, err
	}

	YamlString = utils.ModifyCommonCR(string(buf), cr)

	authNamespace := cr.Namespace
	proxyServerImage := "dellemc/csm-authorization-proxy:v1.4.0"
	opaImage := "openpolicyagent/opa"
	opaKubeMgmtImage := "openpolicyagent/kube-mgmt:0.11"
	tenantServiceImage := "dellemc/csm-authorization-tenant:v1.4.0"
	roleServiceImage := "dellemc/csm-authorization-role:v1.4.0"
	storageServiceImage := "dellemc/csm-authorization-storage:v1.4.0"
	logLevel := "debug"
	zipkinCollectorUri := ""
	zipkinProbability := ""
	redisImage := "redis:6.0.8-alpine"
	redisCommanderImage := "rediscommander/redis-commander:latest"

	redisStorageClass := utils.GetRedisStorage(auth)
	if err != nil {
		return redisStorageClass, err
	}

	for _, component := range auth.Components {
		if component.Name == AuthProxyServerComponent {
			if component.Image != "" {
				if strings.Contains(AuthNamespace, component.Name) {
					authNamespace = string(component.Name)
				} else if strings.Contains(AuthServerImage, component.Name) {
					proxyServerImage = string(component.Image)
				} else if strings.Contains(AuthOpaImage, component.Name) {
					opaImage = string(component.Image)
				} else if strings.Contains(AuthOpaKubeMgmtImage, component.Name) {
					opaKubeMgmtImage = string(component.Image)
				} else if strings.Contains(AuthTenantServiceImage, component.Name) {
					tenantServiceImage = string(component.Image)
				} else if strings.Contains(AuthRoleServiceImage, component.Name) {
					roleServiceImage = string(component.Image)
				} else if strings.Contains(AuthStorageServiceImage, component.Name) {
					storageServiceImage = string(component.Image)
				} else if strings.Contains(AuthRedisImage, component.Name) {
					redisImage = string(component.Image)
				} else if strings.Contains(AuthRedisCommanderImage, component.Name) {
					redisCommanderImage = string(component.Image)
				}
			}
			for _, env := range component.Envs {
				if strings.Contains(AuthLogLevel, env.Name) {
					logLevel = env.Value
				} else if strings.Contains(AuthZipkinCollectorUri, env.Name) {
					zipkinCollectorUri = env.Value
				} else if strings.Contains(AuthZipkinProbability, env.Name) {
					zipkinProbability = env.Value
				} else if strings.Contains(AuthRedisStorageClass, env.Name) {
					redisStorageClass = env.Value
				}
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, AuthNamespace, authNamespace)
	YamlString = strings.ReplaceAll(YamlString, AuthServerImage, proxyServerImage)
	YamlString = strings.ReplaceAll(YamlString, AuthOpaImage, opaImage)
	YamlString = strings.ReplaceAll(YamlString, AuthOpaKubeMgmtImage, opaKubeMgmtImage)
	YamlString = strings.ReplaceAll(YamlString, AuthTenantServiceImage, tenantServiceImage)
	YamlString = strings.ReplaceAll(YamlString, AuthRoleServiceImage, roleServiceImage)
	YamlString = strings.ReplaceAll(YamlString, AuthStorageServiceImage, storageServiceImage)
	YamlString = strings.ReplaceAll(YamlString, AuthLogLevel, logLevel)
	YamlString = strings.ReplaceAll(YamlString, AuthZipkinCollectorUri, zipkinCollectorUri)
	YamlString = strings.ReplaceAll(YamlString, AuthZipkinProbability, zipkinProbability)
	YamlString = strings.ReplaceAll(YamlString, AuthRedisImage, redisImage)
	YamlString = strings.ReplaceAll(YamlString, AuthRedisCommanderImage, redisCommanderImage)
	YamlString = strings.ReplaceAll(YamlString, AuthRedisStorageClass, redisStorageClass)

	return YamlString, nil
}

// AuthorizationServer - apply/delete deployment objects
func AuthorizationServer(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	YamlString, err := getAuthorizationServerDeployment(op, cr, csmv1.Module{})
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

// getAuthorizationIngressRules - update ingress manifest
func getAuthorizationIngressRules(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return YamlString, err
	}

	deploymentPath := fmt.Sprintf("%s/moduleconfig/authorization/%s/%s", op.ConfigDirectory, auth.ConfigVersion, AuthIngressManifest)
	buf, err := os.ReadFile(filepath.Clean(deploymentPath))
	if err != nil {
		return YamlString, err
	}

	YamlString = utils.ModifyCommonCR(string(buf), cr)

	authNamespace := cr.Namespace
	authHostname := "csm-authorization.com"
	proxyIngressClassName := "nginx"
	tenantIngressClassName := "nginx"
	roleIngressClassName := "nginx"
	storageIngressClassName := "nginx"
	proxyIngressHost := utils.GetProxyIngressHost(auth)
	if err != nil {
		return proxyIngressHost, err
	}

	for _, component := range auth.Components {
		if component.Name == AuthProxyServerComponent {
			for _, env := range component.Envs {
				if strings.Contains(AuthNamespace, component.Name) {
					authNamespace = string(component.Name)
				} else if strings.Contains(AuthProxyHost, env.Name) {
					authHostname = env.Value
				} else if strings.Contains(AuthProxyIngressHost, env.Name) {
					proxyIngressHost = env.Value
				} else if strings.Contains(AuthProxyIngressClassName, env.Name) {
					proxyIngressClassName = env.Value
				} else if strings.Contains(AuthTenantIngressClassName, env.Name) {
					tenantIngressClassName = env.Value
				} else if strings.Contains(AuthRoleIngressClassName, env.Name) {
					roleIngressClassName = env.Value
				} else if strings.Contains(AuthStorageIngressClassName, env.Name) {
					storageIngressClassName = env.Value
				}
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, AuthNamespace, authNamespace)
	YamlString = strings.ReplaceAll(YamlString, AuthProxyHost, authHostname)
	YamlString = strings.ReplaceAll(YamlString, AuthProxyIngressHost, proxyIngressHost)
	YamlString = strings.ReplaceAll(YamlString, AuthProxyIngressClassName, proxyIngressClassName)
	YamlString = strings.ReplaceAll(YamlString, AuthTenantIngressClassName, tenantIngressClassName)
	YamlString = strings.ReplaceAll(YamlString, AuthRoleIngressClassName, roleIngressClassName)
	YamlString = strings.ReplaceAll(YamlString, AuthStorageIngressClassName, storageIngressClassName)

	return YamlString, nil
}

// AuthorizationIngress - apply/delete ingress objects
func AuthorizationIngress(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	YamlString, err := getAuthorizationIngressRules(op, cr)
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

func getCertManager(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return YamlString, err
	}

	certManagerPath := fmt.Sprintf("%s/moduleconfig/authorization/%s/%s", op.ConfigDirectory, auth.ConfigVersion, AuthCertManagerManifest)
	buf, err := os.ReadFile(filepath.Clean(certManagerPath))
	if err != nil {
		return YamlString, err
	}

	authNamespace := cr.Namespace

	for _, component := range auth.Components {
		if component.Name == AuthProxyServerComponent {
			if strings.Contains(AuthNamespace, component.Name) {
				authNamespace = string(component.Name)
			}
		}
	}

	YamlString = utils.ModifyCommonCR(string(buf), cr)
	YamlString = strings.ReplaceAll(YamlString, AuthNamespace, authNamespace)

	return YamlString, nil
}

// AuthorizationCertManager - apply/delete cert-manager objects
func AuthorizationCertManager(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient crclient.Client) error {
	YamlString, err := getCertManager(op, cr)
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

// getNginxIngressController - install nginx ingress controller
func getNginxIngressController(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return YamlString, err
	}

	nginxIngressPath := fmt.Sprintf("%s/moduleconfig/authorization/%s/%s", op.ConfigDirectory, auth.ConfigVersion, AuthNginxIngressManifest)
	buf, err := os.ReadFile(filepath.Clean(nginxIngressPath))
	if err != nil {
		return YamlString, err
	}

	authNamespace := cr.Namespace

	for _, component := range auth.Components {
		if component.Name == AuthProxyServerComponent {
			if strings.Contains(AuthNamespace, component.Name) {
				authNamespace = string(component.Name)
			}
		}
	}

	YamlString = utils.ModifyCommonCR(string(buf), cr)
	YamlString = strings.ReplaceAll(YamlString, AuthNamespace, authNamespace)

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

// getPolicies -
func getPolicies(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	auth, err := getAuthorizationModule(cr)
	if err != nil {
		return YamlString, err
	}

	policyPath := fmt.Sprintf("%s/moduleconfig/authorization/%s/%s", op.ConfigDirectory, auth.ConfigVersion, AuthPolicyManifest)
	buf, err := os.ReadFile(filepath.Clean(policyPath))
	if err != nil {
		return YamlString, err
	}

	authNamespace := cr.Namespace

	for _, component := range auth.Components {
		if component.Name == AuthProxyServerComponent {
			if strings.Contains(AuthNamespace, component.Name) {
				authNamespace = string(component.Name)
			}
		}
	}

	YamlString = utils.ModifyCommonCR(string(buf), cr)
	YamlString = strings.ReplaceAll(YamlString, AuthNamespace, authNamespace)
	return YamlString, nil
}

// InstallPolicies -
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
