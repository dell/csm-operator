package drivers

import (
	"context"
	"fmt"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/logger"
	utils "github.com/dell/csm-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/yaml"
)

var defaultVolumeConfigName = map[csmv1.DriverType]string{
	csmv1.PowerScaleName: "isilon-configs",
}

// GetController get controller yaml
func GetController(ctx context.Context, cr csmv1.ContainerStorageModule, controlleryml string, driverType csmv1.DriverType, operatorConfig *utils.OperatorConfig) (*utils.ControllerYAML, error) {
	log := logger.GetLogger(ctx)
	if len(controlleryml) < 1 {
		return nil, fmt.Errorf("error getting controller yaml string")
	}

	yamlString := utils.ModifyCommonCR(controlleryml, cr)

	driverYAML, err := utils.GetDriverYaml(yamlString, "Deployment")
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

		log.Infow("debug container", "name", *c.Name, "image", c.Image)
		removeContainer := false
		for _, s := range cr.Spec.Driver.SideCars {
			if s.Name == *c.Name {
				if s.Enabled == nil {
					log.Infow("Container to be enabled", "name", *c.Name)
					break
				} else if !*s.Enabled {
					removeContainer = true
					log.Infow("Container to be removed", "name", *c.Name)
				} else {
					log.Infow("Container to be enabled", "name", *c.Name)
				}
				break
			}
		}
		if !removeContainer {
			utils.ReplaceAllContainerImageApply(operatorConfig.K8sSidecars, &containers[i])
			utils.UpdateSideCarApply(cr.Spec.Driver.SideCars, &containers[i])
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
		if *v.Name == defaultVolumeConfigName[driverType] && cr.Spec.Driver.AuthSecret != "" {
			controllerYAML.Deployment.Spec.Template.Spec.Volumes[i].Secret.SecretName = &cr.Spec.Driver.AuthSecret
		}

	}

	return &controllerYAML, nil

}

// GetNode get node yaml
func GetNode(ctx context.Context, cr csmv1.ContainerStorageModule, nodeyml string, driverType csmv1.DriverType, operatorConfig *utils.OperatorConfig) (*utils.NodeYAML, error) {
	log := logger.GetLogger(ctx)

	if len(nodeyml) < 1 {
		return nil, fmt.Errorf("error getting node yaml string")
	}

	yamlString := utils.ModifyCommonCR(nodeyml, cr)
	driverYAML, err := utils.GetDriverYaml(yamlString, "DaemonSet")

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

		utils.ReplaceAllContainerImageApply(operatorConfig.K8sSidecars, &containers[i])
		utils.UpdateSideCarApply(cr.Spec.Driver.SideCars, &containers[i])

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
		if *v.Name == defaultVolumeConfigName[driverType] && cr.Spec.Driver.AuthSecret != "" {
			nodeYaml.DaemonSetApplyConfig.Spec.Template.Spec.Volumes[i].Secret.SecretName = &cr.Spec.Driver.AuthSecret
		}

	}

	return &nodeYaml, nil

}

// GetConfigMap get configmap
func GetConfigMap(ctx context.Context, cr csmv1.ContainerStorageModule, configmapYml string) (*corev1.ConfigMap, error) {
	log := logger.GetLogger(ctx)

	if len(configmapYml) < 1 {
		log.Error("GetConfigMap failed", "Error")
		return nil, fmt.Errorf("error getting configmap yaml string")
	}
	yamlString := utils.ModifyCommonCR(configmapYml, cr)

	var configMap corev1.ConfigMap
	err := yaml.Unmarshal([]byte(yamlString), &configMap)
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
func GetCSIDriver(ctx context.Context, cr csmv1.ContainerStorageModule, csidriverYml string) (*storagev1.CSIDriver, error) {
	log := logger.GetLogger(ctx)
	if len(csidriverYml) < 1 {
		return nil, fmt.Errorf("error getting csidriver yaml string")
	}

	var csidriver storagev1.CSIDriver
	err := yaml.Unmarshal([]byte(csidriverYml), &csidriver)
	if err != nil {
		log.Errorw("GetCSIDriver yaml marshall failed", "Error", err.Error())
		return nil, err
	}

	if cr.Spec.Driver.CSIDriverSpec.FSGroupPolicy != "" {
		fsGroupPolicy := storagev1.NoneFSGroupPolicy
		if cr.Spec.Driver.CSIDriverSpec.FSGroupPolicy == "ReadWriteOnceWithFSType" {
			fsGroupPolicy = storagev1.ReadWriteOnceWithFSTypeFSGroupPolicy
		} else if cr.Spec.Driver.CSIDriverSpec.FSGroupPolicy == "File" {
			fsGroupPolicy = storagev1.FileFSGroupPolicy
		}
		csidriver.Spec.FSGroupPolicy = &fsGroupPolicy
		log.Debugw("GetCSIDriver", "fsGroupPolicy", fsGroupPolicy)
	}

	return &csidriver, nil
}
