// Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package modules

import (
	"context"
	"fmt"
	"strings"

	csmv1 "github.com/dell/csm-operator/api/v1"
	drivers "github.com/dell/csm-operator/pkg/drivers"
	"github.com/dell/csm-operator/pkg/logger"
	utils "github.com/dell/csm-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (

	// ObservabilityTopologyName - component topology
	ObservabilityTopologyName string = "topology"

	// ObservabilityMetricsPowerScaleName - component metrics-powerscale
	ObservabilityMetricsPowerScaleName string = "metrics-powerscale"

	// ObservabilityMetricsPowerFlexName - component metrics-powerflex
	ObservabilityMetricsPowerFlexName string = "metrics-powerflex"

	// TopologyLogLevel -
	TopologyLogLevel string = "<TOPOLOGY_LOG_LEVEL>"

	// TopologyImage -
	TopologyImage string = "<TOPOLOGY_IMAGE>"

	// TopologyYamlFile -
	TopologyYamlFile string = "karavi-topology.yaml"
)

// ObservabilitySupportedDrivers is a map containing the CSI Drivers supported by CMS Replication. The key is driver name and the value is the driver plugin identifier
var ObservabilitySupportedDrivers = map[string]SupportedDriverParam{
	"powerscale": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	},
	"isilon": {
		PluginIdentifier:              drivers.PowerScalePluginIdentifier,
		DriverConfigParamsVolumeMount: drivers.PowerScaleConfigParamsVolumeMount,
	},
}

// ObservabilityPrecheck  - runs precheck for CSM Observability
func ObservabilityPrecheck(ctx context.Context, op utils.OperatorConfig, obs csmv1.Module, cr csmv1.ContainerStorageModule, r utils.ReconcileCSM) error {
	log := logger.GetLogger(ctx)

	if _, ok := ObservabilitySupportedDrivers[string(cr.Spec.Driver.CSIDriverType)]; !ok {
		return fmt.Errorf("CSM Operator does not suport Observability deployment for %s driver", cr.Spec.Driver.CSIDriverType)
	}

	// check if provided version is supported
	if obs.ConfigVersion != "" {
		err := checkVersion(string(csmv1.Observability), obs.ConfigVersion, op.ConfigDirectory)
		if err != nil {
			return err
		}
	}

	log.Infof("\nperformed pre checks for: %s", obs.Name)
	return nil
}

// ObservabilityTopology - delete or update topology objects
func ObservabilityTopology(ctx context.Context, isDeleting bool, op utils.OperatorConfig, cr csmv1.ContainerStorageModule, ctrlClient client.Client) error {
	YamlString, err := getTopology(op, cr)
	if err != nil {
		return err
	}

	topoObjects, err := utils.GetObservabilityComponentObj([]byte(YamlString))
	if err != nil {
		return err
	}

	for _, ctrlObj := range topoObjects {
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

// getTopology - get topology yaml string
func getTopology(op utils.OperatorConfig, cr csmv1.ContainerStorageModule) (string, error) {
	YamlString := ""

	obs, err := getObservabilityModule(cr)
	if err != nil {
		return YamlString, err
	}

	buf, err := readConfigFile(obs, cr, op, TopologyYamlFile)
	if err != nil {
		return YamlString, err
	}
	YamlString = string(buf)

	logLevel := "INFO"
	topologyImage := "dellemc/csm-topology:v1.3.0"

	for _, component := range obs.Components {
		if component.Name == ObservabilityTopologyName {
			if component.Image != "" {
				topologyImage = string(component.Image)
			}
			for _, env := range component.Envs {
				if strings.Contains(TopologyLogLevel, env.Name) {
					logLevel = env.Value
				}
			}
		}
	}

	YamlString = strings.ReplaceAll(YamlString, TopologyLogLevel, logLevel)
	YamlString = strings.ReplaceAll(YamlString, TopologyImage, topologyImage)
	return YamlString, nil
}

// getObservabilityModule - get instance of observability module
func getObservabilityModule(cr csmv1.ContainerStorageModule) (csmv1.Module, error) {
	for _, m := range cr.Spec.Modules {
		if m.Name == csmv1.Observability {
			return m, nil

		}
	}
	return csmv1.Module{}, fmt.Errorf("could not find observability module")
}
