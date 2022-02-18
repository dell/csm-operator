package drivers

import (
	"context"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/tests/shared"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	//
	csm                         = csmWithTolerations()
	fakeDriver csmv1.DriverType = "fakeDriver"
	badDriver  csmv1.DriverType = "badDriver"

	// where to find all the yaml files
	config = utils.OperatorConfig{
		ConfigDirectory: "../../test/config",
	}

	tests = []struct {
		// every single unit test name
		name string
		// csm object
		csm csmv1.ContainerStorageModule
		// driver name
		driverName csmv1.DriverType
		// yaml file name to read
		filename string
		// expected error
		expectedErr string
	}{
		{"happy path", csm, csmv1.PowerScaleName, "node.yaml", ""},
		{"file does not exist", csm, fakeDriver, "NonExist.yaml", "no such file or directory"},
		{"config file is invalid", csm, badDriver, "bad.yaml", "unmarshal"},
	}

	opts = zap.Options{
		Development: true,
	}

	// logger = zap.New(zap.UseFlagOptions(&opts)).WithName("pkg/drivers").WithName("unit-test")

	trueBool  bool = true
	falseBool bool = false
)

func TestGetCsiDriver(t *testing.T) {
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetCSIDriver(ctx, tt.csm, config, tt.driverName)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

func TestGetConfigMap(t *testing.T) {
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetConfigMap(ctx, tt.csm, config, tt.driverName)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

func TestGetController(t *testing.T) {
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetController(ctx, tt.csm, config, tt.driverName)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

func TestGetNode(t *testing.T) {
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetNode(ctx, tt.csm, config, tt.driverName, tt.filename)
			if tt.expectedErr == "" {
				assert.Nil(t, err)
			} else {
				assert.Containsf(t, err.Error(), tt.expectedErr, "expected error containing %q, got %s", tt.expectedErr, err)
			}
		})
	}
}

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

	// Add DNS Policy for GetNode test
	res.Spec.Driver.DNSPolicy = "ThisIsADNSPolicy"

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
	}
	sideCarObjEnabledFalse := csmv1.ContainerTemplate{
		Name:    "resizer",
		Enabled: &falseBool,
	}
	sideCarObjEnabledTrue := csmv1.ContainerTemplate{
		Name:    "provisioner",
		Enabled: &trueBool,
	}
	sideCarList := []csmv1.ContainerTemplate{sideCarObjEnabledNil, sideCarObjEnabledFalse, sideCarObjEnabledTrue}
	res.Spec.Driver.SideCars = sideCarList

	return res
}
