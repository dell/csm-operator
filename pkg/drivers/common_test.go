// Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package drivers

import (
	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/tests/shared"
	corev1 "k8s.io/api/core/v1"
)

var (
	// where to find all the yaml files
	config = utils.OperatorConfig{
		ConfigDirectory: "../../tests/config",
	}

	pflexCSMName              = "pflex-csm"
	pflexCredsName            = pflexCSMName + "-config"
	pFlexNS                   = "pflex-test"

	trueBool  bool = true
	falseBool bool = false
)

// makes a csm object with tolerations
func csmWithTolerations(driver csmv1.DriverType, version string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add tolerations to controller and node
	res.Spec.Driver.Node.Tolerations = []corev1.Toleration{
		{
			Key:               "123",
			Value:             "123",
			TolerationSeconds: new(int64),
		},
	}
	res.Spec.Driver.Controller.Tolerations = []corev1.Toleration{
		{
			Key:               "123",
			Value:             "123",
			TolerationSeconds: new(int64),
		},
	}

	// Add FSGroupPolicy
	res.Spec.Driver.CSIDriverSpec.FSGroupPolicy = "File"

	// Add FSGroupPolicy
	res.Spec.Driver.CSIDriverSpec.FSGroupPolicy = "ReadWriteOnceWithFSType"	

	// Add DNS Policy for GetNode test
	res.Spec.Driver.DNSPolicy = "ThisIsADNSPolicy"

	// Add image name
	res.Spec.Driver.Common.Image = "thisIsAnImage"

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = version

	// Add pscale driver type
	res.Spec.Driver.CSIDriverType = driver

	// Add NodeSelector to node and controller
	res.Spec.Driver.Node.NodeSelector = map[string]string{"thisIs": "NodeSelector"}
	res.Spec.Driver.Controller.NodeSelector = map[string]string{"thisIs": "NodeSelector"}

	// Add log level to cover some code in GetConfigMap
	envVarLogLevel := corev1.EnvVar{Name: "CSI_LOG_LEVEL"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVarLogLevel}
	// Add sidecars to trigger code in controller
	sideCarObjEnabledNil := csmv1.ContainerTemplate{
		Name:    "driver",
		Enabled: nil,
		Args:    []string{"--v=5"},
	}
	sideCarObjEnabledFalse := csmv1.ContainerTemplate{
		Name:    "resizer",
		Enabled: &falseBool,
		Args:    []string{"--v=5"},
	}
	sideCarObjEnabledTrue := csmv1.ContainerTemplate{
		Name:    "provisioner",
		Enabled: &trueBool,
		Args:    []string{"--volume-name-prefix=k8s"},
	}
	sideCarList := []csmv1.ContainerTemplate{sideCarObjEnabledNil, sideCarObjEnabledFalse, sideCarObjEnabledTrue}
	res.Spec.Driver.SideCars = sideCarList

	return res
}

// JJL make a functino that uses the standard name and calls this function
// makes a pflex csm object
func csmForPowerFlex(customCSMName string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM(customCSMName, pFlexNS, shared.PFlexConfigVersion)

	// Add sdc initcontainer
	res.Spec.Driver.InitContainers = []csmv1.ContainerTemplate{csmv1.ContainerTemplate{
		Name:            "sdc",
		Enabled:         &trueBool,
		Image:           "image",
		ImagePullPolicy: "IfNotPresent",
		Args:            []string{},
		Envs:            []corev1.EnvVar{corev1.EnvVar{Name: "MDM"}},
		Tolerations:     []corev1.Toleration{},
	}}

	// Add sdc-monitor Sidecar
	res.Spec.Driver.SideCars = []csmv1.ContainerTemplate{csmv1.ContainerTemplate{
		Name:            "sdc-monitor",
		Enabled:         &falseBool,
		Image:           "image",
		ImagePullPolicy: "IfNotPresent",
		Args:            []string{},
		Envs:            []corev1.EnvVar{corev1.EnvVar{Name: "MDM"}},
		Tolerations:     []corev1.Toleration{},
	}}

	//res.Spec.Driver.CSIDriverSpec.FSGroupPolicy == "ReadWriteOnceWithFSType"

	// Add pflex driver version
	res.Spec.Driver.ConfigVersion = shared.PFlexConfigVersion
	res.Spec.Driver.CSIDriverType = csmv1.PowerFlex

	return res
}

