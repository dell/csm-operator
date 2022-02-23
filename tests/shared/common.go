package shared

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	csmv1 "github.com/dell/csm-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ConfigVersion - used in all unit test
	ConfigVersion string = "v2.1.0"
)

// StorageKey is used to store a runtime object. It's used for both clientgo client and controller runtime client
type StorageKey struct {
	Namespace string
	Name      string
	Kind      string
}

// ErrorInjector is used for testing errors for the fake client
type ErrorInjector interface {
	ShouldFail(method string, obj runtime.Object) error
}

// GetKey returns the storageKey based on the given runtime object
func GetKey(obj runtime.Object) (StorageKey, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return StorageKey{}, err
	}

	gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
	if err != nil {
		return StorageKey{}, err
	}

	return StorageKey{
		Name:      accessor.GetName(),
		Namespace: accessor.GetNamespace(),
		Kind:      gvk.Kind,
	}, nil
}

// MakeCSM returns a csm from given params
func MakeCSM(name, ns, configVersion string) csmv1.ContainerStorageModule {

	driverObj := MakeDriver(configVersion, "true")

	csmObj := csmv1.ContainerStorageModule{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: make(map[string]string),
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Driver: driverObj,
		},
		Status: csmv1.ContainerStorageModuleStatus{},
	}

	return csmObj
}

// MakeDriver returns a driver object from given params
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

// MakeSecret  returns a driver pre-req secret array-config
func MakeSecret(name, ns, configVersion string) *corev1.Secret {
	data := map[string][]byte{
		"config": []byte("csm"),
	}
	object := metav1.ObjectMeta{Name: name, Namespace: ns}
	secret := &corev1.Secret{Data: data, ObjectMeta: object}
	return secret
}

// MakePod returns a pod object
func MakePod(name, ns string) corev1.Pod {
	podObj := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{},
		},
	}

	return podObj
}
