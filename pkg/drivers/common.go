package drivers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	utils "github.com/dell/csm-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/yaml"
)


// GetController get controller yaml
func GetController(cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, driverName csmv1.DriverType) (*utils.ControllerYAML, error) {
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/controller.yaml", operatorConfig.ConfigDirectory, driverName, cr.Spec.Driver.ConfigVersion)
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
			// for _, e := range containers[i].Env {
			// 	if e.Value != nil {
			// 		//Log.Info("resolved 2 ", "env", *e.Name, "value", *e.Value)
			// 	}
			// }
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

// GetNode get node yaml
func GetNode(cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, driverName csmv1.DriverType) (*utils.NodeYAML, error) {
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/node.yaml", operatorConfig.ConfigDirectory, driverName, cr.Spec.Driver.ConfigVersion)
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
			// for _, e := range containers[i].Env {
			// 	if e.Value != nil {
			// 		//Log.Info("resolved 2 ", "env", *e.Name, "value", *e.Value)
			// 	}
			// }
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

// GetConfigMap get configmap
func GetConfigMap(cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, driverName csmv1.DriverType) (*corev1.ConfigMap, error) {
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/driver-config-params.yaml", operatorConfig.ConfigDirectory, driverName, cr.Spec.Driver.ConfigVersion)

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
func GetCSIDriver(cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, driverName csmv1.DriverType) (*storagev1.CSIDriver, error) {
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/csidriver.yaml", operatorConfig.ConfigDirectory, driverName, cr.Spec.Driver.ConfigVersion)
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
