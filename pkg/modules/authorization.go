package modules

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

const (
	//DefaultPluginIdentifier placeholder constant for driver
	DefaultPluginIdentifier = "<DriverPluginIdentifier>"
)

// SupportedDrivers is a map containing the CSI Drivers supported by CMS Authorization. The key is driver name and the value is the driver plugin identifier
var SupportedDrivers = map[string]string{
	"powerscale": "powerscale", "isilon": "powerscale", // either powerscale or isilon are valid types
}

func getAuthCR(cr csmv1.ContainerStorageModule, op utils.OperatorConfig) (*csmv1.Module, *corev1.Container, error) {
	var err error
	configMapName := map[string]string{
		"powerscale": "csi-isilon",
		"isilon":     "csi-isilon",
	}
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
	buf, err := ioutil.ReadFile(configMapPath)
	if err != nil {
		return nil, nil, err
	}

	YamlString := string(buf)
	if cr.Name != "" {
		YamlString = strings.ReplaceAll(YamlString, utils.DefaultReleaseName, configMapName[string(cr.Spec.Driver.CSIDriverType)])
	}

	if string(cr.Spec.Driver.Common.ImagePullPolicy) != "" {
		YamlString = strings.ReplaceAll(YamlString, utils.DefaultImagePullPolicy, string(cr.Spec.Driver.Common.ImagePullPolicy))
	}

	YamlString = strings.ReplaceAll(YamlString, DefaultPluginIdentifier, SupportedDrivers[string(cr.Spec.Driver.CSIDriverType)])

	var container corev1.Container
	err = yaml.Unmarshal([]byte(YamlString), &container)
	if err != nil {
		return nil, nil, err
	}

	skipCertValid := false
	for _, env := range authModule.Components[0].Envs {
		if env.Name == "INSECURE" {
			skipCertValid, _ = strconv.ParseBool(env.Value)
		}
	}

	if skipCertValid { // do not mount proxy-server-root-certificate
		for i, c := range container.VolumeMounts {
			if c.Name == "proxy-server-root-certificate" {
				container.VolumeMounts[i] = container.VolumeMounts[len(container.VolumeMounts)-1]
				container.VolumeMounts = container.VolumeMounts[:len(container.VolumeMounts)-1]
			}

		}

	}

	return &authModule, &container, nil

}

func getAuthVolumes(cr csmv1.ContainerStorageModule, op utils.OperatorConfig, auth csmv1.ContainerTemplate) ([]corev1.Volume, error) {
	version, err := utils.GetModuleDefaultVersion(cr.Spec.Driver.ConfigVersion, cr.Spec.Driver.CSIDriverType, csmv1.Authorization, op.ConfigDirectory)
	if err != nil {
		return nil, err
	}

	configMapPath := fmt.Sprintf("%s/moduleconfig/authorization/%s/volumes.yaml", op.ConfigDirectory, version)
	buf, err := ioutil.ReadFile(configMapPath)
	if err != nil {
		return nil, err
	}

	var vols []corev1.Volume
	err = yaml.Unmarshal(buf, &vols)
	if err != nil {
		return nil, err
	}

	skipCertValid := false
	for _, env := range auth.Envs {
		if env.Name == "INSECURE" {
			skipCertValid, _ = strconv.ParseBool(env.Value)
		}
	}

	if skipCertValid { // do not mount proxy-server-root-certificate
		for i, c := range vols {
			if c.Name == "proxy-server-root-certificate" {
				vols[i] = vols[len(vols)-1]
				return vols[:len(vols)-1], nil

			}

		}

	}
	return vols, nil
}

// AuthInjectDaemonset
func AuthInjectDaemonset(ds appsv1.DaemonSet, cr csmv1.ContainerStorageModule, op utils.OperatorConfig) (*appsv1.DaemonSet, error) {
	authModule, containerPtr, err := getAuthCR(cr, op)
	if err != nil {
		return nil, err
	}

	container := *containerPtr
	container = utils.UpdateSideCar(authModule.Components, container)

	vols, err := getAuthVolumes(cr, op, authModule.Components[0])
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

// AuthInjectDeployment
func AuthInjectDeployment(dp appsv1.Deployment, cr csmv1.ContainerStorageModule, op utils.OperatorConfig) (*appsv1.Deployment, error) {
	authModule, containerPtr, err := getAuthCR(cr, op)
	if err != nil {
		return nil, err
	}

	container := *containerPtr
	container = utils.UpdateSideCar(authModule.Components, container)

	vols, err := getAuthVolumes(cr, op, authModule.Components[0])
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

// AuthorizationPrecheck
func AuthorizationPrecheck(ctx context.Context, cr *csmv1.ContainerStorageModule, auth csmv1.Module, r utils.ReconcileCSM, log logr.Logger) error {
	if _, ok := SupportedDrivers[string(cr.Spec.Driver.CSIDriverType)]; !ok {
		return fmt.Errorf("csm authorization does not support %s driver", string(cr.Spec.Driver.CSIDriverType))
	}

	// Check for secrets
	skipCertValid := false
	for _, env := range auth.Components[0].Envs {
		if env.Name == "INSECURE" {
			b, err := strconv.ParseBool(env.Value)
			if err != nil {
				return fmt.Errorf("%s is an invalid value for INSECURE: %v", env.Value, err)
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
		err := r.GetClient().Get(ctx, types.NamespacedName{Name: name,
			Namespace: cr.GetNamespace()}, found)
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("failed to find secret %s and certificate validation is requested", name)
			}
			log.Error(err, "Failed to query for secret. Warning - the controller pod may not start")
		}
	}

	// TODO(Michael): Do Other configuration checks if needed

	return nil
}
