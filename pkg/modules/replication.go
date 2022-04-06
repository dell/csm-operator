package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"

	"github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/logger"
	utils "github.com/dell/csm-operator/pkg/utils"
	rbacv1 "k8s.io/api/rbac/v1"

	applyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	acorev1 "k8s.io/client-go/applyconfigurations/core/v1"

	"sigs.k8s.io/yaml"
)

const (
	RepctlBinary                    = "repctl"
	ReplicationPrefix               = "replication.storage.dell.com"
	DefaultReplicationContextPrefix = "<ReplicationContextPrefix>"
	DefaultReplicationPrefix        = "<ReplicationPrefix>"
)

var (
	XCSIReplicaCTXPrefix = "X_CSI_REPLICATION_CONTEXT_PREFIX"
	XCSIReplicaPrefix    = "X_CSI_REPLICATION_PREFIX"
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

func getRepctlPrefices(replicaModule csmv1.Module, driverType csmv1.DriverType) (string, string) {
	replicationPrefix := ReplicationPrefix
	replicationContextPrefix := ReplicationSupportedDrivers[string(driverType)].PluginIdentifier

	for _, component := range replicaModule.Components {
		if component.Name == "dell-csi-replicator" {
			for _, env := range component.Envs {
				fmt.Println(env.Name, env.Value)
				if env.Name == XCSIReplicaPrefix {
					replicationPrefix = env.Value
				} else if env.Name == XCSIReplicaCTXPrefix {
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

	buf, err := readConfigFile(replicaModule, cr, op, "container.yaml")
	if err != nil {
		return nil, nil, err
	}

	YamlString := utils.ModifyCommonCR(string(buf), cr)

	replicationContextPrefix, replicationPrefix := getRepctlPrefices(replicaModule, cr.Spec.Driver.CSIDriverType)
	YamlString = strings.ReplaceAll(YamlString, DefaultReplicationPrefix, replicationPrefix)
	YamlString = strings.ReplaceAll(YamlString, DefaultReplicationContextPrefix, replicationContextPrefix)
	YamlString = strings.ReplaceAll(YamlString, DefaultDriverConfigParamsVolumeMount, ReplicationSupportedDrivers[string(cr.Spec.Driver.CSIDriverType)].DriverConfigParamsVolumeMount)

	var container acorev1.ContainerApplyConfiguration
	err = yaml.Unmarshal([]byte(YamlString), &container)
	if err != nil {
		return nil, nil, err
	}

	return &replicaModule, &container, nil
}

// ReplicationInjectDeployment - inject replication into deployment
func ReplicationInjectDeployment(dp applyv1.DeploymentApplyConfiguration, cr csmv1.ContainerStorageModule, op utils.OperatorConfig) (*applyv1.DeploymentApplyConfiguration, error) {
	replicaModule, containerPtr, err := getReplicaApplyCR(cr, op)
	if err != nil {
		return nil, err
	}
	container := *containerPtr
	dp.Spec.Template.Spec.Containers = append(dp.Spec.Template.Spec.Containers, container)

	// inject replication in driver environment

	replicationContextPrefix, replicationPrefix := getRepctlPrefices(*replicaModule, cr.Spec.Driver.CSIDriverType)
	for i, cnt := range dp.Spec.Template.Spec.Containers {
		if *cnt.Name == "driver" {
			dp.Spec.Template.Spec.Containers[i].Env = append(dp.Spec.Template.Spec.Containers[i].Env,
				acorev1.EnvVarApplyConfiguration{Name: &XCSIReplicaCTXPrefix, Value: &replicationContextPrefix},
				acorev1.EnvVarApplyConfiguration{Name: &XCSIReplicaPrefix, Value: &replicationPrefix},
			)
			break
		}
	}
	return &dp, nil
}

// CheckApplyContainersReplica --
func CheckApplyContainersReplica(contianers []acorev1.ContainerApplyConfiguration, cr csmv1.ContainerStorageModule) error {
	replicaModule := csmv1.Module{}
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Replication {
			replicaModule = m
			break
		}
	}

	replicaString := "dell-csi-replicator"
	driverString := "driver"
	replicationContextPrefix, replicationPrefix := getRepctlPrefices(replicaModule, cr.Spec.Driver.CSIDriverType)
	for _, cnt := range contianers {
		if *cnt.Name == replicaString {
			// check volumes
			volName := ReplicationSupportedDrivers[string(cr.Spec.Driver.CSIDriverType)].DriverConfigParamsVolumeMount
			foundVol := false
			for _, vol := range cnt.VolumeMounts {
				if *vol.Name == volName {
					foundVol = true
					break
				}
			}
			if !foundVol {
				return fmt.Errorf("missing the following volume mount %s", volName)
			}

			//check arguments
			foundReplicationPrefix := false
			foundReplicationContextPrefix := false
			for _, arg := range cnt.Args {
				if fmt.Sprintf("--context-prefix=%s", replicationContextPrefix) == arg {
					foundReplicationContextPrefix = true
				}
				if fmt.Sprintf("--prefix=%s", replicationPrefix) == arg {
					foundReplicationPrefix = true
				}
			}
			if !foundReplicationContextPrefix {
				return fmt.Errorf("missing the following  argument %s", replicationContextPrefix)
			}
			if !foundReplicationPrefix {
				return fmt.Errorf("missing the following  argument %s", replicationPrefix)
			}

		} else if *cnt.Name == driverString {
			foundReplicationPrefix := false
			foundReplicationContextPrefix := false
			for _, env := range cnt.Env {
				if *env.Name == XCSIReplicaPrefix {
					foundReplicationPrefix = true
					if *env.Value != replicationPrefix {
						return fmt.Errorf("expected %s to have a value of: %s but got: %s", XCSIReplicaPrefix, replicationPrefix, *env.Value)
					}
				}
				if *env.Name == XCSIReplicaCTXPrefix {
					foundReplicationContextPrefix = true
					if *env.Value != replicationContextPrefix {
						return fmt.Errorf("expected %s to have a value of: %s but got: %s", XCSIReplicaCTXPrefix, replicationContextPrefix, *env.Value)
					}
				}
			}
			if !foundReplicationContextPrefix {
				return fmt.Errorf("missing the following  argument %s", replicationContextPrefix)
			}
			if !foundReplicationPrefix {
				return fmt.Errorf("missing the following  argument %s", replicationPrefix)
			}

		}
	}
	return nil
}

// CheckClusterRoleReplica -
func CheckClusterRoleReplica(rules []rbacv1.PolicyRule) error {
	foundRepilcaGroup := false
	foundReplicaStatus := false
	for _, rule := range rules {
		if len(rule.APIGroups) > 0 && rule.APIGroups[0] == "replication.storage.dell.com" {
			if rule.Resources[0] == "dellcsireplicationgroups" {
				foundRepilcaGroup = true
			}
			if rule.Resources[0] == "dellcsireplicationgroups/status" {
				foundReplicaStatus = true
			}
		}
	}

	if !foundRepilcaGroup {
		return fmt.Errorf("missing the resources for %s", "dellcsireplicationgroups")
	}
	if !foundReplicaStatus {
		return fmt.Errorf("missing the resources for %s", "dellcsireplicationgroups/status")
	}
	return nil

}

// ReplicationInjectClusterRole - inject replication into clusterrole
func ReplicationInjectClusterRole(clusterRole rbacv1.ClusterRole, cr csmv1.ContainerStorageModule, op utils.OperatorConfig) (*rbacv1.ClusterRole, error) {
	var err error
	replicaModule := csmv1.Module{}
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Replication {
			replicaModule = m
			break
		}
	}
	buf, err := readConfigFile(replicaModule, cr, op, "rules.yaml")
	if err != nil {
		return nil, err
	}

	var rules []rbacv1.PolicyRule
	err = yaml.Unmarshal(buf, &rules)
	if err != nil {
		return nil, err
	}

	clusterRole.Rules = append(clusterRole.Rules, rules...)
	return &clusterRole, nil
}

// ReplicationPrecheck  - runs precheck for CSM ReplicationPrecheck
func ReplicationPrecheck(ctx context.Context, op utils.OperatorConfig, replica csmv1.Module, cr csmv1.ContainerStorageModule, r utils.ReconcileCSM) error {
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

	replicaClusters, err := utils.GetDefaultClusters(ctx, cr, r)
	if err != nil {
		return err
	}

	for _, cluster := range replicaClusters {
		switch cr.Spec.Driver.CSIDriverType {
		case csmv1.PowerScale:
			tmpCR := cr
			err := drivers.PrecheckPowerScale(ctx, &tmpCR, cluster.ClutsterCTRLClient)
			if err != nil {
				return fmt.Errorf("failed powerscale validation: %v for cluster %s", err, cluster.ClutsterID)
			}
		}
	}
	return nil
}
