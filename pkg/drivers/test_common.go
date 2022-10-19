package drivers

import (
	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/tests/shared"
	corev1 "k8s.io/api/core/v1"
)

var (
	csm = csmWithTolerations()

	// where to find all the yaml files
	config = utils.OperatorConfig{
		ConfigDirectory: "../../tests/config",
	}

	trueBool  bool = true
	falseBool bool = false
)

// makes a csm object with tolerations
func csmWithTolerations() csmv1.ContainerStorageModule {
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

	// Add DNS Policy for GetNode test
	res.Spec.Driver.DNSPolicy = "ThisIsADNSPolicy"

	// Add image name
	res.Spec.Driver.Common.Image = "thisIsAnImage"

	// Add pscale driver version
	res.Spec.Driver.ConfigVersion = "v2.2.0"

	// Add pscale driver version
	res.Spec.Driver.CSIDriverType = csmv1.PowerScale

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

