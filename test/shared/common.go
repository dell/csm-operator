package shared

import (
	"k8s.io/apimachinery/pkg/runtime"

	csmv1 "github.com/dell/csm-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var Scheme = runtime.NewScheme()

func MakeCSM(name, ns string) csmv1.ContainerStorageModule {

	driverObj := MakeDriver("v2.0.0", "true")

	csmObj := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: driverObj,
		},
		Status: csmv1.ContainerStorageModuleStatus{},
	}

	return csmObj
}

func MakeDriver(configVersion, skipCertValid string) csmv1.Driver {
	driverObj := csmv1.Driver{
		ConfigVersion: configVersion,
		Common: csmv1.ContainerTemplate{
			Envs: []corev1.EnvVar{
				{
					Name:  "X_CSI_ISI_SKIP_CERTIFICATE_VALIDATION",
					Value: skipCertValid,
				},
			},
		},
	}

	return driverObj
}
