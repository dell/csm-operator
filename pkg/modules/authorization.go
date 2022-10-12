package modules

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
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

// AuthorizationSupportedDrivers is a map containing the CSI Drivers supported by CMS Authorization. The key is driver name and the value is the driver plugin identifier
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
	buf, err := ioutil.ReadFile(filepath.Clean(configMapPath))
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
	buf, err := ioutil.ReadFile(filepath.Clean(configMapPath))
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

	return nil
}
