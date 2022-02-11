package drivers

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/logger"
	utils "github.com/dell/csm-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/yaml"
)

// GetController get controller yaml
func GetController(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, driverName csmv1.DriverType) (*utils.ControllerYAML, error) {
	log := logger.GetLogger(ctx)
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/controller.yaml", operatorConfig.ConfigDirectory, driverName, cr.Spec.Driver.ConfigVersion)
	buf, err := ioutil.ReadFile(configMapPath)
	if err != nil {
		log.Errorw("GetController failed", "Error", err.Error())
		return nil, err
	}

	YamlString := utils.ModifyCommonCR(string(buf), cr)

	driverYAML, err := utils.GetDriverYaml(YamlString, "Deployment")
	if err != nil {
		log.Errorw("GetController get Deployment failed", "Error", err.Error())
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
	newcontainers := make([]acorev1.ContainerApplyConfiguration, 0)
	for i, c := range containers {
		if string(*c.Name) == "driver" {
			containers[i].Env = utils.ReplaceAllApplyCustomEnvs(c.Env, cr.Spec.Driver.Common.Envs, cr.Spec.Driver.Controller.Envs)
			c.Env = containers[i].Env
			if string(cr.Spec.Driver.Common.Image) != "" {
				image := string(cr.Spec.Driver.Common.Image)
				c.Image = &image
			}
		}

		removeContainer := false
		for _, s := range cr.Spec.Driver.SideCars {
			if s.Name == *c.Name {
				if s.Enabled == nil {
					fmt.Printf("Container  to be enabled : %s\n", *c.Name)
					break

				} else if !*s.Enabled {
					removeContainer = true
					fmt.Printf("Container to be removed : %s\n", *c.Name)
				} else {
					fmt.Printf("Container to be enabled : %s\n", *c.Name)
				}
				break
			}
		}
		if !removeContainer {
			utils.ReplaceAllContainerImageApply(operatorConfig.K8sVersion, &c)
			utils.UpdateSideCarApply(cr.Spec.Driver.SideCars, &c)
			newcontainers = append(newcontainers, c)
		}

	}

	controllerYAML.Deployment.Spec.Template.Spec.Containers = newcontainers
	// Update volumes
	for i, v := range controllerYAML.Deployment.Spec.Template.Spec.Volumes {
		if *v.Name == "certs" {
			newV, err := getApplyCertVolume(cr)
			if err != nil {
				log.Errorw("GetController spec template volumes", "Error", err.Error())
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
func GetNode(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, driverType csmv1.DriverType, filename string) (*utils.NodeYAML, error) {
	log := logger.GetLogger(ctx)
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/%s", operatorConfig.ConfigDirectory, driverType, cr.Spec.Driver.ConfigVersion, filename)
	buf, err := ioutil.ReadFile(configMapPath)
	if err != nil {
		log.Errorw("GetNode failed", "Error", err.Error())
		return nil, err
	}

	YamlString := utils.ModifyCommonCR(string(buf), cr)

	driverYAML, err := utils.GetDriverYaml(YamlString, "DaemonSet")
	if err != nil {
		log.Errorw("GetNode Daemonset failed", "Error", err.Error())
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
			if string(cr.Spec.Driver.Common.Image) != "" {
				image := string(cr.Spec.Driver.Common.Image)
				containers[i].Image = &image
			}
		}

		utils.ReplaceAllContainerImageApply(operatorConfig.K8sVersion, &c)
		utils.UpdateSideCarApply(cr.Spec.Driver.SideCars, &c)

	}

	nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Containers = containers

	// Update volumes
	for i, v := range nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Volumes {
		if *v.Name == "certs" {
			newV, err := getApplyCertVolume(cr)
			if err != nil {
				log.Errorw("GetNode apply cert Volume failed", "Error", err.Error())
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
func GetConfigMap(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, driverName csmv1.DriverType) (*corev1.ConfigMap, error) {
	log := logger.GetLogger(ctx)
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/driver-config-params.yaml", operatorConfig.ConfigDirectory, driverName, cr.Spec.Driver.ConfigVersion)

	if _, err := os.Stat(configMapPath); os.IsNotExist(err) {
		log.Errorw("GetConfigMap failed", "Error", err.Error())
		return nil, err
	}

	buf, err := ioutil.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		log.Errorw("GetConfigMap failed", "Error", err.Error())
		return nil, err
	}
	YamlString := utils.ModifyCommonCR(string(buf), cr)

	var configMap corev1.ConfigMap
	err = yaml.Unmarshal([]byte(YamlString), &configMap)
	if err != nil {
		log.Errorw("GetConfigMap yaml marshall failed", "Error", err.Error())
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

// GetCSIDriver get driver
func GetCSIDriver(ctx context.Context, cr csmv1.ContainerStorageModule, operatorConfig utils.OperatorConfig, driverName csmv1.DriverType) (*storagev1.CSIDriver, error) {
	log := logger.GetLogger(ctx)
	configMapPath := fmt.Sprintf("%s/driverconfig/%s/%s/csidriver.yaml", operatorConfig.ConfigDirectory, driverName, cr.Spec.Driver.ConfigVersion)
	buf, err := ioutil.ReadFile(configMapPath)
	if err != nil {
		log.Errorw("GetCSIDriver failed", "Error", err.Error())
		return nil, err
	}

	var csidriver storagev1.CSIDriver
	err = yaml.Unmarshal(buf, &csidriver)
	if err != nil {
		log.Errorw("GetCSIDriver yaml marshall failed", "Error", err.Error())
		return nil, err
	}

	return &csidriver, nil
}
