//  Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//       http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package shared

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	csmv1 "github.com/dell/csm-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigVersions used for all unit tests
const (
	ConfigVersion            string = "v2.2.0"
	UpgradeConfigVersion     string = "v2.3.0"
	JumpUpgradeConfigVersion string = "v2.4.0"
	OldConfigVersion         string = "v2.1.0"
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
				{
					Name:  "CHECK_OWNER_REFERENCE",
					Value: "false",
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
