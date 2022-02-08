package drivers

import (
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/dell/csm-operator/test/shared"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	//
	csm = csmWithTolerations()

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
		{"file does not exist", csm, csmv1.PowerScaleName, "NonExist.yaml", "no such file or directory"},
	}

	opts = zap.Options{
		Development: true,
	}

	// logger = zap.New(zap.UseFlagOptions(&opts)).WithName("pkg/drivers").WithName("unit-test")
)

func TestGetNode(t *testing.T) {

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetNode(tt.csm, config, tt.driverName, tt.filename)
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
	res.Spec.Driver.Node.Tolerations = []corev1.Toleration{
		{
			Key:               "123",
			Value:             "123",
			TolerationSeconds: new(int64),
		},
	}

	return res
}
