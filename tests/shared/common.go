//  Copyright © 2021 - 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"os"
	"path/filepath"

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
	PFlexConfigVersion          string = "v2.14.1"
	DowngradeConfigVersion      string = "v2.13.1"
	ConfigVersion               string = "v2.14.0"
	UpgradeConfigVersion        string = "v2.12.0"
	JumpUpgradeConfigVersion    string = "v2.13.0"
	JumpDowngradeConfigVersion  string = "v2.12.0"
	OldConfigVersion            string = "v2.2.0"
	BadConfigVersion            string = "v0"
	PStoreConfigVersion         string = "v2.14.1"
	UnityConfigVersion          string = "v2.14.0"
	PScaleConfigVersion         string = "v2.14.0"
	PmaxConfigVersion           string = "v2.14.1"
	AuthServerConfigVersion     string = "v2.1.0"
	AppMobConfigVersion         string = "v1.1.0"
	ResiliencyCSMConfigVersion  string = "v2.14.1"
	ReplicationCSMConfigVersion string = "v2.14.1"
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

// MakeModuleCSM returns a csm from given params
func MakeModuleCSM(name, ns, configVersion string) csmv1.ContainerStorageModule {
	moduleObj := MakeModule(configVersion)

	csmObj := csmv1.ContainerStorageModule{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: make(map[string]string),
		},
		Spec: csmv1.ContainerStorageModuleSpec{
			Modules: []csmv1.Module{moduleObj},
		},
		Status: csmv1.ContainerStorageModuleStatus{},
	}
	return csmObj
}

// MakeDriver returns a driver object from given params
func MakeDriver(configVersion, skipCertValid string) csmv1.Driver {
	driverObj := csmv1.Driver{
		ConfigVersion: configVersion,
		Common: &csmv1.ContainerTemplate{
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
		Node: &csmv1.ContainerTemplate{
			Envs: []corev1.EnvVar{
				{
					Name:  "X_CSI_SDC_SFTP_REPO_ENABLED",
					Value: "false",
				},
			},
		},
	}

	return driverObj
}

// MakeModule returns a module object from given params
func MakeModule(configVersion string) csmv1.Module {
	moduleObj := csmv1.Module{
		ConfigVersion:     configVersion,
		ForceRemoveModule: true,
		Components:        []csmv1.ContainerTemplate{{}},
	}

	return moduleObj
}

// MakeSecretPowerFlexWithZone  returns a driver pre-req secret with zoning specified
func MakeSecretPowerFlexWithZone(name, ns, _ string) *corev1.Secret {
	dataWithZone := `
- username: "admin"
  password: "password"
  systemID: "2b11bb111111bb1b"
  endpoint: "https://127.0.0.2"
  skipCertificateValidation: true
  mdm: "10.0.0.3,10.0.0.4"
  zone:
    name: "US-EAST"
    labelKey: "zone.csi-vxflexos.dellemc.com"
`
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string][]byte{
			"config": []byte(dataWithZone),
		},
	}
	return secret
}

// MakeSecretPowerFlex  returns a pflex driver pre-req secret
func MakeSecretPowerFlex(name, ns, _ string) *corev1.Secret {
	dataWithoutZone := `
- username: "admin"
  password: "password"
  systemID: "2b11bb111111bb1b"
  endpoint: "https://127.0.0.2"
  skipCertificateValidation: true
  mdm: "10.0.0.3,10.0.0.4"
`

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string][]byte{
			"config": []byte(dataWithoutZone),
		},
	}
	return secret
}

// MakeSecretPowerFlexMultiZoneInvalid  returns a pflex driver pre-req secret with invalid zone config
func MakeSecretPowerFlexMultiZoneInvalid(name, ns, _ string) *corev1.Secret {
	dataWithInvalidZone := `
- username: "admin"
  password: "password"
  systemID: "2b11bb111111bb1b"
  endpoint: "https://127.0.0.2"
  skipCertificateValidation: true
  mdm: "10.0.0.3,10.0.0.4"
- username: "admin"
  password: "password"
  systemID: "2b11bb111111bb1b"
  endpoint: "https://127.0.0.2"
  skipCertificateValidation: true
  mdm: "10.0.0.3,10.0.0.4"
  zone:
    name: "US-EAST"
    labelKey: "myzone.csi-vxflexos.dellemc.com"
`

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string][]byte{
			"config": []byte(dataWithInvalidZone),
		},
	}
	return secret
}

// MakeSecret  returns a driver pre-req secret array-config
func MakeSecret(name, ns, _ string) *corev1.Secret {
	data := map[string][]byte{
		"config": []byte("csm"),
	}
	object := metav1.ObjectMeta{Name: name, Namespace: ns}
	secret := &corev1.Secret{Data: data, ObjectMeta: object}
	return secret
}

// MakeConfigMap returns a driver pre-req configmap array-config
func MakeConfigMap(name, ns, _ string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string]string{
			"data": name,
		},
	}
}

// MakeSecretWithJSON returns a driver pre-req secret array-config
func MakeSecretWithJSON(name string, ns string, configFile string) *corev1.Secret {
	configJSON, _ := os.ReadFile(filepath.Clean(configFile)) // #nosec G304
	data := map[string][]byte{
		"config": configJSON,
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

// MakeNode returns a node object
func MakeNode(name, ns string) corev1.Node {
	nodeObj := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{},
		},
	}

	return nodeObj
}

// MakeReverseProxyModule returns a csireverseproxy object
func MakeReverseProxyModule(_ string) csmv1.Module {
	revproxy := csmv1.Module{
		Name:          csmv1.ReverseProxy,
		Enabled:       true,
		ConfigVersion: "v2.6.0",
		Components: []csmv1.ContainerTemplate{
			{
				Name:  string(csmv1.ReverseProxyServer),
				Image: "dell/proxy:v2.6.0",
				Envs: []corev1.EnvVar{
					{
						Name:  "X_CSI_REVPROXY_TLS_SECRET",
						Value: "csirevproxy-tls-secret",
					},
					{
						Name:  "X_CSI_REVPROXY_PORT",
						Value: "2222",
					},
					{
						Name:  "X_CSI_CONFIG_MAP_NAME",
						Value: "powermax-reverseproxy-config",
					},
				},
			},
		},
		ForceRemoveModule: false,
	}
	return revproxy
}
