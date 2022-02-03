package drivers

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	utils "github.com/dell/csm-operator/pkg/utils"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/yaml"
	// +kubebuilder:scaffold:imports
)

// Log global for all methods in this file
var Log logr.Logger

// GetPowerScaleController get controller yaml
func GetPowerScaleController(cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) (*utils.ControllerYAML, error) {
	configMapPath := fmt.Sprintf("%s/driverconfig/powerscale/%s/controller.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)
	buf, err := ioutil.ReadFile(configMapPath)
	if err != nil {
		return nil, err
	}

	YamlString := utils.ModifyCommonCR(string(buf), cr)

	driverYAML, err := utils.GetDriverYAML(YamlString, "Deployment")
	if err != nil {
		return nil, err
	}

	controllerYAML := driverYAML.(utils.ControllerYAML)
	controllerYAML.Deployment.Spec.Replicas = &cr.Spec.Driver.Replicas

	if len(cr.Spec.Driver.Controller.Tolerations) != 0 {
		tols := make([]acorev1.TolerationApplyConfiguration, 0)
		for _, t := range cr.Spec.Driver.Controller.Tolerations {
			toleration := acorev1.Toleration()
			toleration.WithEffect(t.Effect)
			toleration.WithKey(t.Key)
			toleration.WithValue(t.Value)
			toleration.WithOperator(t.Operator)
			toleration.WithTolerationSeconds(*t.TolerationSeconds)
			tols = append(tols, *toleration)
		}

		controllerYAML.Deployment.Spec.Template.Spec.Tolerations = tols
	}

	if cr.Spec.Driver.Controller.NodeSelector != nil {
		controllerYAML.Deployment.Spec.Template.Spec.NodeSelector = cr.Spec.Driver.Controller.NodeSelector
	}

	containers := controllerYAML.Deployment.Spec.Template.Spec.Containers
	for i, c := range containers {
		if string(*c.Name) == "driver" {
			containers[i].Env = utils.ReplaceAllApplyCustomEnvs(c.Env, cr.Spec.Driver.Common.Envs, cr.Spec.Driver.Controller.Envs)
			for _, e := range containers[i].Env {
				if e.Value != nil {
					//Log.Info("resolved 2 ", "env", *e.Name, "value", *e.Value)
				}
			}
			if string(cr.Spec.Driver.Common.Image) != "" {
				image := string(cr.Spec.Driver.Common.Image)
				containers[i].Image = &image
			}
		}

		tmp := utils.ReplaceALLContainerImageApply(operatorConfig.K8sVersion, containers[i])
		containers[i] = utils.UpdateSideCarApply(cr.Spec.Driver.SideCars, tmp)

	}

	controllerYAML.Deployment.Spec.Template.Spec.Containers = containers
	// Update volumes
	for i, v := range controllerYAML.Deployment.Spec.Template.Spec.Volumes {
		if *v.Name == "certs" {
			newV, err := getApplyCertVolume(cr)
			if err != nil {
				return nil, err
			}
			controllerYAML.Deployment.Spec.Template.Spec.Volumes[i] = *newV
		}
		if *v.Name == cr.Name+"-creds" && cr.Spec.Driver.AuthSecret != "" {
			controllerYAML.Deployment.Spec.Template.Spec.Volumes[i].Secret.SecretName = &cr.Spec.Driver.AuthSecret
		}

	}

	return &controllerYAML, nil

}

// GetPowerScaleNode get node yaml
func GetPowerScaleNode(cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) (*utils.NodeYAML, error) {
	configMapPath := fmt.Sprintf("%s/driverconfig/powerscale/%s/node.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)
	buf, err := ioutil.ReadFile(configMapPath)
	if err != nil {
		return nil, err
	}

	YamlString := utils.ModifyCommonCR(string(buf), cr)

	driverYAML, err := utils.GetDriverYAML(YamlString, "DaemonSet")
	if err != nil {
		return nil, err
	}

	nodeYaml := driverYAML.(utils.NodeYAML)

	if cr.Spec.Driver.DNSPolicy != "" {
		dnspolicy := corev1.DNSPolicy(cr.Spec.Driver.DNSPolicy)
		nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.DNSPolicy = &dnspolicy
	}

	if len(cr.Spec.Driver.Node.Tolerations) != 0 {
		tols := make([]acorev1.TolerationApplyConfiguration, 0)
		for _, t := range cr.Spec.Driver.Node.Tolerations {
			toleration := acorev1.Toleration()
			toleration.WithEffect(t.Effect)
			toleration.WithKey(t.Key)
			toleration.WithValue(t.Value)
			toleration.WithOperator(t.Operator)
			toleration.WithTolerationSeconds(*t.TolerationSeconds)
			tols = append(tols, *toleration)
		}

		nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Tolerations = tols
	}

	if cr.Spec.Driver.Node.NodeSelector != nil {
		nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.NodeSelector = cr.Spec.Driver.Node.NodeSelector
	}

	containers := nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Containers
	for i, c := range containers {
		if string(*c.Name) == "driver" {
			containers[i].Env = utils.ReplaceAllApplyCustomEnvs(c.Env, cr.Spec.Driver.Common.Envs, cr.Spec.Driver.Node.Envs)
			for _, e := range containers[i].Env {
				if e.Value != nil {
					//Log.Info("resolved 2 ", "env", *e.Name, "value", *e.Value)
				}
			}
			if string(cr.Spec.Driver.Common.Image) != "" {
				image := string(cr.Spec.Driver.Common.Image)
				containers[i].Image = &image
			}
		}

		tmp := utils.ReplaceALLContainerImageApply(operatorConfig.K8sVersion, containers[i])
		containers[i] = utils.UpdateSideCarApply(cr.Spec.Driver.SideCars, tmp)
	}

	nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Containers = containers

	// Update volumes
	for i, v := range nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Volumes {
		if *v.Name == "certs" {
			newV, err := getApplyCertVolume(cr)
			if err != nil {
				return nil, err
			}
			nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Volumes[i] = *newV
		}
		if *v.Name == cr.Name+"-creds" && cr.Spec.Driver.AuthSecret != "" {
			nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Volumes[i].Secret.SecretName = &cr.Spec.Driver.AuthSecret
		}

	}

	return &nodeYaml, nil

}

// GetPowerScaleConfigMap get configmap
func GetPowerScaleConfigMap(cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) (*corev1.ConfigMap, error) {
	configMapPath := fmt.Sprintf("%s/driverconfig/powerscale/%s/driver-config-params.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)

	if _, err := os.Stat(configMapPath); os.IsNotExist(err) {
		return nil, err
	}

	buf, err := ioutil.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		return nil, err
	}
	YamlString := utils.ModifyCommonCR(string(buf), cr)

	var configMap corev1.ConfigMap
	err = yaml.Unmarshal([]byte(YamlString), &configMap)
	if err != nil {
		return nil, err
	}

	for _, env := range cr.Spec.Driver.Common.Envs {
		if env.Name == "CSI_LOG_LEVEL" {
			configMap.Data = map[string]string{
				"driver-config-params.yaml": fmt.Sprintf("%s: %s", env.Name, env.Value),
			}
			break
		}

	}

	return &configMap, nil

}

// GetPowerScaleCSIDriver get driver
func GetPowerScaleCSIDriver(cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) (*storagev1.CSIDriver, error) {
	configMapPath := fmt.Sprintf("%s/driverconfig/powerscale/%s/csidriver.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)
	buf, err := ioutil.ReadFile(configMapPath)
	if err != nil {
		return nil, err
	}

	var csidriver storagev1.CSIDriver
	err = yaml.Unmarshal(buf, &csidriver)
	if err != nil {
		return nil, err
	}

	return &csidriver, nil

}

// PrecheckPowerScale do input validation
func PrecheckPowerScale(ctx context.Context, cr *csmv1.ContainerStorageModule, r utils.ReconcileCSM, log logr.Logger) error {
	// Check for secrete only
	config := cr.Name + "-creds"

	if cr.Spec.Driver.AuthSecret != "" {
		config = cr.Spec.Driver.AuthSecret
	}

	// check if skip validation is enabled:
	skipCertValid := false
	certCount := 1
	for _, env := range cr.Spec.Driver.Common.Envs {
		if env.Name == "X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION" {
			b, err := strconv.ParseBool(env.Value)
			if err != nil {
				return fmt.Errorf("%s is an invalid value for X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION: %v", env.Value, err)
			}
			skipCertValid = b
		}
		if env.Name == "CERT_SECRET_COUNT" {
			d, err := strconv.ParseInt(env.Value, 0, 8)
			if err != nil {
				return fmt.Errorf("%s is an invalid value for CERT_SECRET_COUNT: %v", env.Value, err)
			}
			certCount = int(d)
		}
	}

	secrets := []string{config}
	if !skipCertValid {
		for i := 0; i < certCount; i++ {
			secrets = append(secrets, fmt.Sprintf("%s-certs-%d", cr.Name, i))
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
	}

	// TODO(Michael): Do Other configuration checks

	return nil
}

func getApplyCertVolume(cr csmv1.ContainerStorageModule) (*acorev1.VolumeApplyConfiguration, error) {
	skipCertValid := false
	certCount := 1
	for _, env := range cr.Spec.Driver.Common.Envs {
		if env.Name == "X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION" {
			b, err := strconv.ParseBool(env.Value)
			if err != nil {
				return nil, fmt.Errorf("%s is an invalid value for X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION: %v", env.Value, err)
			}
			skipCertValid = b
		}
		if env.Name == "CERT_SECRET_COUNT" {
			d, err := strconv.ParseInt(env.Value, 0, 8)
			if err != nil {
				return nil, fmt.Errorf("%s is an invalid value for CERT_SECRET_COUNT: %v", env.Value, err)
			}
			certCount = int(d)
		}
	}

	name := "certs"
	volume := acorev1.VolumeApplyConfiguration{
		Name: &name,
		VolumeSourceApplyConfiguration: acorev1.VolumeSourceApplyConfiguration{
			Projected: &acorev1.ProjectedVolumeSourceApplyConfiguration{
				Sources: []acorev1.VolumeProjectionApplyConfiguration{},
			},
		},
	}

	if !skipCertValid {
		for i := 0; i < certCount; i++ {
			localname := fmt.Sprintf("%s-certs-%d", cr.Name, i)
			value := fmt.Sprintf("cert-%d", i)
			source := acorev1.SecretProjectionApplyConfiguration{
				LocalObjectReferenceApplyConfiguration: acorev1.LocalObjectReferenceApplyConfiguration{Name: &localname},
				Items: []acorev1.KeyToPathApplyConfiguration{
					{
						Key:  &value,
						Path: &value,
					},
				},
			}
			volume.VolumeSourceApplyConfiguration.Projected.Sources = append(volume.VolumeSourceApplyConfiguration.Projected.Sources, acorev1.VolumeProjectionApplyConfiguration{Secret: &source})

		}
	}

	return &volume, nil

}
