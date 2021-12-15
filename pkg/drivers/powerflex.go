package drivers

import (
	"fmt"
	"io/ioutil"

	csmv1 "github.com/dell/csm-operator/api/v1"
	utils "github.com/dell/csm-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/yaml"
)

// GetPowerFlexConfigMap returns the logging config map from the given cr
func GetPowerFlexConfigMap(cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) (*corev1.ConfigMap, error) {
	configMapPath := fmt.Sprintf("%s/driverconfig/powerflex/%s/driver-config-params.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)
	buf, err := ioutil.ReadFile(configMapPath)
	if err != nil {
		return nil, err
	}
	yamlString := utils.ModifyCommonCR(string(buf), cr)

	var configMap corev1.ConfigMap
	err = yaml.Unmarshal([]byte(yamlString), &configMap)
	if err != nil {
		return nil, err
	}

	for _, env := range cr.Spec.Driver.Common.Envs {
		if env.Name == "CSI_LOG_LEVEL" {
			configMap.Data = map[string]string{
				"driver-config-params.yaml": fmt.Sprintf("%s: %s", env.Name, env.Value),
			}
		}
	}

	return &configMap, nil
}

// GetPowerFlexCSIDriver returns the csidriver struct from the given cr
func GetPowerFlexCSIDriver(cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) (*storagev1.CSIDriver, error) {
	configMapPath := fmt.Sprintf("%s/driverconfig/powerflex/%s/csidriver.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)
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

// GetPowerFlexNode returns the node yaml from the given cr
func GetPowerFlexNode(cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) (*utils.NodeYAML, error) {
	configMapPath := fmt.Sprintf("%s/driverconfig/powerflex/%s/node.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)
	buf, err := ioutil.ReadFile(configMapPath)
	if err != nil {
		return nil, err
	}

	yamlString := utils.ModifyCommonCR(string(buf), cr)

	driverYaml, err := utils.GetDriverYAML(yamlString, "DaemonSet")
	if err != nil {
		return nil, err
	}

	nodeYaml := driverYaml.(utils.NodeYAML)

	if cr.Spec.Driver.DNSPolicy != "" {
		nodeYaml.DaemonSet.Spec.Template.Spec.DNSPolicy = corev1.DNSPolicy(cr.Spec.Driver.DNSPolicy)
	}

	if len(cr.Spec.Driver.Node.Tolerations) != 0 {
		nodeYaml.DaemonSet.Spec.Template.Spec.Tolerations = cr.Spec.Driver.Node.Tolerations
	}

	if cr.Spec.Driver.Node.NodeSelector != nil {
		nodeYaml.DaemonSet.Spec.Template.Spec.NodeSelector = cr.Spec.Driver.Node.NodeSelector
	}

	containers := nodeYaml.DaemonSet.Spec.Template.Spec.Containers
	for i, c := range containers {
		if string(c.Name) == "driver" {
			env := utils.ReplaceAllEnvs(c.Env, cr.Spec.Driver.Common.Envs)
			containers[i].Env = utils.ReplaceAllEnvs(env, cr.Spec.Driver.Node.Envs)

			image := string(cr.Spec.Driver.Common.Image)
			if image != "" {
				containers[i].Image = image
			}
		}

		tmp := utils.ReplaceALLContainerImage(operatorConfig.K8sVersion, containers[i])
		containers[i] = utils.UpdateSideCar(cr.Spec.Driver.SideCars, tmp)
	}

	nodeYaml.DaemonSet.Spec.Template.Spec.Containers = containers

	// TODO: update volumes for creds

	return &nodeYaml, nil
}

// GetPowerFlexController returns the controller yaml from the given cr
func GetPowerFlexController(cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig) (*utils.ControllerYAML, error) {

	configMapPath := fmt.Sprintf("%s/driverconfig/powerFlex/%s/controller.yaml", operatorConfig.ConfigDirectory, cr.Spec.Driver.ConfigVersion)
	buf, err := ioutil.ReadFile(configMapPath)
	if err != nil {
		return nil, err
	}

	yamlString := utils.ModifyCommonCR(string(buf), cr)

	driverYaml, err := utils.GetDriverYAML(yamlString, "Deployment")
	if err != nil {
		return nil, err
	}

	controllerYaml := driverYaml.(utils.ControllerYAML)
	controllerYaml.Deployment.Spec.Replicas = &cr.Spec.Driver.Replicas

	if len(cr.Spec.Driver.Controller.Tolerations) != 0 {
		controllerYaml.Deployment.Spec.Template.Spec.Tolerations = cr.Spec.Driver.Controller.Tolerations
	}

	if cr.Spec.Driver.Controller.NodeSelector != nil {
		controllerYaml.Deployment.Spec.Template.Spec.NodeSelector = cr.Spec.Driver.Controller.NodeSelector
	}

	containers := controllerYaml.Deployment.Spec.Template.Spec.Containers
	for i, c := range containers {
		if string(c.Name) == "driver" {
			env := utils.ReplaceAllEnvs(c.Env, cr.Spec.Driver.Common.Envs)
			containers[i].Env = utils.ReplaceAllEnvs(env, cr.Spec.Driver.Controller.Envs)
			if string(cr.Spec.Driver.Common.Image) != "" {
				containers[i].Image = string(cr.Spec.Driver.Common.Image)
			}
		}

		c = utils.ReplaceALLContainerImage(operatorConfig.K8sVersion, c)
		containers[i] = utils.UpdateSideCar(cr.Spec.Driver.SideCars, c)
	}

	controllerYaml.Deployment.Spec.Template.Spec.Containers = containers

	// TODO: update volumes like certs

	return &controllerYaml, nil
}
