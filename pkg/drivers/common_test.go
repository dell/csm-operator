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

	pflexCSMName   = "pflex-csm"
	pflexCredsName = pflexCSMName + "-config"
	pFlexNS        = "pflex-test"

	trueBool  bool = true
	falseBool bool = false
)

// makes a csm object with tolerations
func csmWithTolerations(driver csmv1.DriverType, version string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add tolerations to controller and node
	res.Spec.Driver.Node.Tolerations = []corev1.Toleration{
		{
			Key:               "notNil",
			Value:             "123",
			TolerationSeconds: new(int64),
		},
		{
			Key:               "nil",
			Value:             "123",
			TolerationSeconds: nil,
		},
	}
	res.Spec.Driver.Controller.Tolerations = []corev1.Toleration{
		{
			Key:               "notNil",
			Value:             "123",
			TolerationSeconds: new(int64),
		},
		{
			Key:               "nil",
			Value:             "123",
			TolerationSeconds: nil,
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

	// Add CSI_LOG_LEVEL environment variables
	envVar := corev1.EnvVar{Name: "CSI_LOG_LEVEL"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVar}

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

func csmWithPowerstore(driver csmv1.DriverType, version string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add FSGroupPolicy
	res.Spec.Driver.CSIDriverSpec.FSGroupPolicy = "File"

	// Add DNS Policy for GetNode test
	res.Spec.Driver.DNSPolicy = "ThisIsADNSPolicy"

	// Add image name
	res.Spec.Driver.Common.Image = "thisIsAnImage"

	// Add pstore driver version
	res.Spec.Driver.ConfigVersion = version

	// Add pstore driver type
	res.Spec.Driver.CSIDriverType = driver

	// Add NodeSelector to node and controller
	res.Spec.Driver.Node.NodeSelector = map[string]string{"thisIs": "NodeSelector"}
	res.Spec.Driver.Controller.NodeSelector = map[string]string{"thisIs": "NodeSelector"}

	// Add node name prefix to cover some code in GetNode
	nodeNamePrefix := corev1.EnvVar{Name: "X_CSI_POWERSTORE_NODE_NAME_PREFIX"}

	// Add FC port filter
	fcFilterPath := corev1.EnvVar{Name: "X_CSI_FC_PORTS_FILTER_FILE_PATH"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{nodeNamePrefix, fcFilterPath}

	// Add node fields specific to powerstore
	enableChap := corev1.EnvVar{Name: "X_CSI_POWERSTORE_ENABLE_CHAP", Value: "true"}
	healthMonitor := corev1.EnvVar{Name: "X_CSI_HEALTH_MONITOR_ENABLED", Value: "true"}
	maxVolumesPerNode := corev1.EnvVar{Name: "X_CSI_POWERSTORE_MAX_VOLUMES_PER_NODE", Value: "0"}
	res.Spec.Driver.Node.Envs = []corev1.EnvVar{enableChap, healthMonitor, maxVolumesPerNode}

	// Add controller fields specific
	nfsAclsParam := corev1.EnvVar{Name: "X_CSI_NFS_ACLS"}
	externalAccess := corev1.EnvVar{Name: "X_CSI_POWERSTORE_EXTERNAL_ACCESS"}
	res.Spec.Driver.Controller.Envs = []corev1.EnvVar{nfsAclsParam, healthMonitor, externalAccess}

	res.Spec.Driver.CSIDriverSpec.StorageCapacity = true

	return res
}

func csmWithPowermax(driver csmv1.DriverType, version string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", version)

	// Add FSGroupPolicy
	res.Spec.Driver.CSIDriverSpec.FSGroupPolicy = "ReadWriteOnceWithFSType"

	// Add DNS Policy for GetNode test
	res.Spec.Driver.DNSPolicy = "ThisIsADNSPolicy"

	// Add image name
	res.Spec.Driver.Common.Image = "thisIsAnImage"

	// Add pstore driver version
	res.Spec.Driver.ConfigVersion = version

	// Add pstore driver type
	res.Spec.Driver.CSIDriverType = driver

	// Add NodeSelector to node and controller
	res.Spec.Driver.Node.NodeSelector = map[string]string{"thisIs": "NodeSelector"}
	res.Spec.Driver.Controller.NodeSelector = map[string]string{"thisIs": "NodeSelector"}

	// Add common envs
	commonEnvs := getPmaxCommonEnvs()
	res.Spec.Driver.Common.Envs = commonEnvs

	// Add node fields specific to powermax
	enableChap := corev1.EnvVar{Name: "X_CSI_POWERMAX_ISCSI_ENABLE_CHAP", Value: "true"}
	healthMonitor := corev1.EnvVar{Name: "X_CSI_HEALTH_MONITOR_ENABLED", Value: "true"}
	nodeTopology := corev1.EnvVar{Name: "X_CSI_TOPOLOGY_CONTROL_ENABLED", Value: "true"}
	res.Spec.Driver.Node.Envs = []corev1.EnvVar{enableChap, healthMonitor, nodeTopology}

	// Add controller fields specific to powermax
	res.Spec.Driver.Controller.Envs = []corev1.EnvVar{healthMonitor}

	// Add CSI Driver specific fields
	res.Spec.Driver.CSIDriverSpec.StorageCapacity = true

	// Add reverseproxy module
	revproxy := shared.MakeReverseProxyModule(shared.ConfigVersion)
	res.Spec.Modules = []csmv1.Module{revproxy}
	return res
}

func getPmaxCommonEnvs() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "X_CSI_MANAGED_ARRAYS",
			Value: "00000000001",
		},
		{
			Name:  "X_CSI_POWERMAX_ENDPOINT",
			Value: "hhtps:/u4p.123:8443",
		},
		{
			Name:  "X_CSI_K8S_CLUSTER_PREFIX",
			Value: "TST",
		},
		{
			Name:  "X_CSI_POWERMAX_DEBUG",
			Value: "false",
		},
		{
			Name:  "X_CSI_POWERMAX_PORTGROUPS",
			Value: "pg",
		},
		{
			Name:  "X_CSI_TRANSPORT_PROTOCOL",
			Value: "",
		},
		{
			Name:  "X_CSI_VSPHERE_ENABLED",
			Value: "false",
		},
		{
			Name:  "X_CSI_VSPHERE_PORTGROUP",
			Value: "vpg",
		},
		{
			Name:  "X_CSI_VSPHERE_HOSTNAME",
			Value: "vHN",
		},
		{
			Name:  "X_CSI_VCENTER_HOST",
			Value: "vH",
		},
		{
			Name:  "X_CSI_VSPHERE_ENABLED",
			Value: "false",
		},
		{
			Name:  "X_CSI_IG_MODIFY_HOSTNAME",
			Value: "false",
		},
		{
			Name:  "X_CSI_IG_NODENAME_TEMPLATE",
			Value: "",
		},
	}
}

func csmWithPowerScale(driver csmv1.DriverType, version string) csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add FSGroupPolicy
	res.Spec.Driver.CSIDriverSpec.FSGroupPolicy = "File"

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

	// Add node name prefix to cover some code in GetNode
	nodeNamePrefix := corev1.EnvVar{Name: "X_CSI_POWERSTORE_NODE_NAME_PREFIX"}

	// Add FC port filter
	fcFilterPath := corev1.EnvVar{Name: "X_CSI_FC_PORTS_FILTER_FILE_PATH"}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{nodeNamePrefix, fcFilterPath}

	// Add environment variable
	csiLogLevel := corev1.EnvVar{Name: "CSI_LOG_LEVEL", Value: "debug"}

	res.Spec.Driver.Node.Envs = []corev1.EnvVar{csiLogLevel}

	// Add node fields specific to powerstore
	healthMonitor := corev1.EnvVar{Name: "X_CSI_HEALTH_MONITOR_ENABLED", Value: "true"}
	res.Spec.Driver.Node.Envs = []corev1.EnvVar{healthMonitor}

	// Add controller fields specific
	res.Spec.Driver.Controller.Envs = []corev1.EnvVar{healthMonitor}

	res.Spec.Driver.CSIDriverSpec.StorageCapacity = true

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
		Args:    []string{"--volume-name-prefix=k8s", "--enable-capacity=true", "--capacity-ownerref-level=2"},
	}
	sideCarList := []csmv1.ContainerTemplate{sideCarObjEnabledNil, sideCarObjEnabledFalse, sideCarObjEnabledTrue}
	res.Spec.Driver.SideCars = sideCarList
	return res
}

func csmWithUnity(driver csmv1.DriverType, version string, certProvided bool) csmv1.ContainerStorageModule {
	res := shared.MakeCSM("csm", "driver-test", shared.ConfigVersion)

	// Add FSGroupPolicy
	res.Spec.Driver.CSIDriverSpec.FSGroupPolicy = "File"

	// Add DNS Policy for GetNode test
	res.Spec.Driver.DNSPolicy = "ThisIsADNSPolicy"

	// Add image name
	res.Spec.Driver.Common.Image = "thisIsAnImage"

	// Add unity driver version
	res.Spec.Driver.ConfigVersion = version

	// Add unity driver type
	res.Spec.Driver.CSIDriverType = driver

	// Add NodeSelector to node and controller
	res.Spec.Driver.Node.NodeSelector = map[string]string{"thisIs": "NodeSelector"}
	res.Spec.Driver.Controller.NodeSelector = map[string]string{"thisIs": "NodeSelector"}

	// Add environment variables
	envVar1 := corev1.EnvVar{Name: "X_CSI_UNITY_ALLOW_MULTI_POD_ACCESS", Value: "false"}
	envVar2 := corev1.EnvVar{Name: "MAX_UNITY_VOLUMES_PER_NODE", Value: "0"}
	envVar3 := corev1.EnvVar{Name: "X_CSI_UNITY_SYNC_NODEINFO_INTERVAL", Value: "15"}
	envVar4 := corev1.EnvVar{Name: "TENANT_NAME", Value: ""}
	envVar5 := corev1.EnvVar{Name: "CSI_LOG_LEVEL", Value: "debug"}
	envVar6 := corev1.EnvVar{Name: "X_CSI_UNITY_SKIP_CERTIFICATE_VALIDATION", Value: "true"}
	envVar7 := corev1.EnvVar{Name: "CERT_SECRET_COUNT", Value: "1"}
	if certProvided {
		envVar6 = corev1.EnvVar{Name: "X_CSI_UNITY_SKIP_CERTIFICATE_VALIDATION", Value: "false"}
	}
	res.Spec.Driver.Common.Envs = []corev1.EnvVar{envVar1, envVar2, envVar3, envVar4, envVar5, envVar6, envVar7}

	// Add node fields specific to unity
	healthMonitor := corev1.EnvVar{Name: "X_CSI_HEALTH_MONITOR_ENABLED", Value: "true"}
	res.Spec.Driver.Node.Envs = []corev1.EnvVar{healthMonitor}

	// Add controller fields specific
	res.Spec.Driver.Controller.Envs = []corev1.EnvVar{healthMonitor}
	// res.Spec.Driver.CSIDriverSpec.StorageCapacity = true

	return res
}
