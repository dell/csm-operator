package modules

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	k8sClient "github.com/dell/csm-operator/k8s"
	"github.com/dell/csm-operator/pkg/drivers"
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
	RepctlBinary                    = "repctl"
	ReplicationControllerNameSpace  = "dell-replication-controller"
	ReplicationPrefix               = "replication.storage.dell.com"
	DefaultReplicationContextPrefix = "<ReplicationContextPrefix>"
	DefaultReplicationPrefix        = "<ReplicationPrefix>"
)

// ReplicationSupportedDrivers is a map containing the CSI Drivers supported by CMS Replication. The key is driver name and the value is the driver plugin identifier
var ReplicationSupportedDrivers = map[string]SupportedDriverParam{
	"powerscale": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	},
	"isilon": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	},
}

func clusterIDs(replica csmv1.Module) ([]string, error) {
	var clusterIDs []string
	for _, comp := range replica.Components {
		if comp.Name == ReplicationControllerNameSpace {
			for _, env := range comp.Envs {
				if env.Name == "CLUSTERS_IDS" {
					clusterIDs = strings.Split(env.Value, ",")
					break
				}
			}
		}
	}
	err := fmt.Errorf("CLUSTERS_IDS on CR should have more than 1 commma seperated cluster IDs. Got  %d", len(clusterIDs))
	if len(clusterIDs) >= 2 {
		err = nil
	}
	return clusterIDs, err
}

func getConfigData(ctx context.Context, clusterID string, ctrlClient crclient.Client) ([]byte, error) {
	log := logger.GetLogger(ctx)
	secret := &corev1.Secret{}
	if err := ctrlClient.Get(ctx, types.NamespacedName{Name: clusterID,
		Namespace: ReplicationControllerNameSpace}, secret); err != nil {
		if k8serrors.IsNotFound(err) {
			return []byte("error"), fmt.Errorf("failed to find secret %s", clusterID)
		}
		log.Error(err, "Failed to query for secret. Warning - the controller pod may not start")
	}
	return secret.Data["data"], nil
}

// NewControllerRuntimeClientWrapper -
var NewControllerRuntimeClientWrapper = func(clusterConfigData []byte) (crclient.Client, error) {
	return k8sClient.NewControllerRuntimeClient(clusterConfigData)
}

func getClusterCtrlClient(ctx context.Context, clusterID string, ctrlClient crclient.Client) (crclient.Client, error) {
	clusterConfigData, err := getConfigData(ctx, clusterID, ctrlClient)
	if err != nil {
		return nil, err
	}

	return NewControllerRuntimeClientWrapper(clusterConfigData)
}

func getRepctlPrefices(replicaModule csmv1.Module, driverType csmv1.DriverType) (string, string) {
	replicationPrefix := ReplicationPrefix
	replicationContextPrefix := ReplicationSupportedDrivers[string(driverType)].PluginIdentifier

	for _, component := range replicaModule.Components {
		if component.Name == "dell-replication-controller" {
			for _, env := range component.Envs {
				if env.Name == "X_CSI_REPLICATION_PREFIX" {
					replicationPrefix = env.Value
				} else if env.Name == "X_CSI_REPLICATION_CONTEXT_PREFIX" {
					replicationContextPrefix = env.Value
				}
			}
		}
	}

	return replicationContextPrefix, replicationPrefix
}

func getReplicaApplyCR(cr csmv1.ContainerStorageModule, op utils.OperatorConfig) (*csmv1.Module, *acorev1.ContainerApplyConfiguration, error) {
	var err error
	replicaModule := csmv1.Module{}
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Replication {
			replicaModule = m
			break
		}
	}

	replicaConfigVersion := replicaModule.ConfigVersion
	if replicaConfigVersion == "" {
		replicaConfigVersion, err = utils.GetModuleDefaultVersion(cr.Spec.Driver.ConfigVersion, cr.Spec.Driver.CSIDriverType, csmv1.Replication, op.ConfigDirectory)
		if err != nil {
			return nil, nil, err
		}
	}

	configMapPath := fmt.Sprintf("%s/moduleconfig/replication/%s/container.yaml", op.ConfigDirectory, replicaConfigVersion)
	buf, err := ioutil.ReadFile(filepath.Clean(configMapPath))
	if err != nil {
		return nil, nil, err
	}

	YamlString := utils.ModifyCommonCR(string(buf), cr)

	replicationContextPrefix, replicationPrefix := getRepctlPrefices(replicaModule, cr.Spec.Driver.CSIDriverType)

	YamlString = strings.ReplaceAll(YamlString, DefaultReplicationPrefix, replicationPrefix)
	YamlString = strings.ReplaceAll(YamlString, DefaultPluginIdentifier, replicationContextPrefix)
	YamlString = strings.ReplaceAll(YamlString, DefaultDriverConfigParamsVolumeMount, ReplicationSupportedDrivers[string(cr.Spec.Driver.CSIDriverType)].DriverConfigParamsVolumeMount)

	fmt.Print(YamlString)

	var container acorev1.ContainerApplyConfiguration
	err = yaml.Unmarshal([]byte(YamlString), &container)
	if err != nil {
		return nil, nil, err
	}
	fmt.Print(container)

	return &replicaModule, &container, nil
}

// ReplicationDeployment - inject replication into deployment
func ReplicationDeployment(dp applyv1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule, op utils.OperatorConfig) (*applyv1.DeploymentApplyConfiguration, error) {
	replicaModule, containerPtr, err := getReplicaApplyCR(cr, op)
	if err != nil {
		return nil, err
	}
	container := *containerPtr
	dp.Spec.Template.Spec.Containers = append(dp.Spec.Template.Spec.Containers, container)

	// inject replication in driver environment
	xcsiReplicaCTXPrefix := "X_CSI_REPLICATION_CONTEXT_PREFIX"
	xcsiReplicaPrefix := "X_CSI_REPLICATION_PREFIX"
	replicationContextPrefix, replicationPrefix := getRepctlPrefices(*replicaModule, cr.Spec.Driver.CSIDriverType)
	for i, cnt := range dp.Spec.Template.Spec.Containers {
		if *cnt.Name == "driver" {
			dp.Spec.Template.Spec.Containers[i].Env = append(dp.Spec.Template.Spec.Containers[i].Env,
				acorev1.EnvVarApplyConfiguration{Name: &xcsiReplicaCTXPrefix, Value: &replicationContextPrefix},
				acorev1.EnvVarApplyConfiguration{Name: &xcsiReplicaPrefix, Value: &replicationPrefix},
			)
			break
		}
	}
	return &dp, nil
}

// ReplicationPrecheck  - runs precheck for CSM ReplicationPrecheck
func ReplicationPrecheck(ctx context.Context, op utils.OperatorConfig, replica csmv1.Module, cr csmv1.ContainerStorageModule, sourceCtrlClient crclient.Client) error {
	log := logger.GetLogger(ctx)

	if _, ok := ReplicationSupportedDrivers[string(cr.Spec.Driver.CSIDriverType)]; !ok {
		return fmt.Errorf("CSM Operator does not suport Replication deployment for %s driver", cr.Spec.Driver.CSIDriverType)
	}

	// Check repctl binary runs fine
	repctlBinary, ok := os.LookupEnv("REPCTL_BINARY")
	if !ok {
		repctlBinary = RepctlBinary
		log.Warnf("REPCTL_BINARY environment variable not defined. Using default %s", repctlBinary)
	}

	if out, err := exec.CommandContext(ctx, repctlBinary, "--help").CombinedOutput(); err != nil {
		log.Errorf("%s", out)
		return fmt.Errorf("repctl not installed: %v", err)
	}

	// check if provided version is supported
	if replica.ConfigVersion != "" {
		err := checkVersion(string(csmv1.Replication), replica.ConfigVersion, op.ConfigDirectory)
		if err != nil {
			return err
		}
	}

	clusterIDs, err := clusterIDs(replica)
	if err != nil {
		return err
	}

	for _, clusterID := range clusterIDs {
		targetCtrlClient, err := getClusterCtrlClient(ctx, clusterID, sourceCtrlClient)
		if err != nil {
			return err
		}

		switch cr.Spec.Driver.CSIDriverType {
		case csmv1.PowerScale:
			tmpCR := cr
			err := drivers.PrecheckPowerScale(ctx, &tmpCR, targetCtrlClient)
			if err != nil {
				return fmt.Errorf("failed powerscale validation: %v for cluster %s", err, clusterID)
			}
		}
	}
	return nil
}
