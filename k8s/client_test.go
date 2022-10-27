/*
 Copyright © 2021 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/
package k8s

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/apimachinery/pkg/version"
	discoveryfake "k8s.io/client-go/discovery/fake"
)

type testOverrides struct {
	getClientSetWrapper func() (kubernetes.Interface, error)
	ignoreError         bool
}

func Test_IsOpenShift(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, testOverrides){
		"success ": func(*testing.T) (bool, testOverrides) {
			return true, testOverrides{
				getClientSetWrapper: func() (kubernetes.Interface, error) {
					fakeClientSet := fake.NewSimpleClientset()
					fakeDiscovery, ok := fakeClientSet.Discovery().(*discoveryfake.FakeDiscovery)
					if !ok {
						t.Fatalf("couldn't convert Discovery() to *FakeDiscovery")
					}
					fakeDiscovery.Resources = []*metav1.APIResourceList{
						{
							APIResources: []metav1.APIResource{
								{Name: "security.openshift.io"},
							},
							GroupVersion: "security.openshift.io/v1",
						},
					}
					return fakeClientSet, nil
				},
			}
		},
		"bad config data ": func(*testing.T) (bool, testOverrides) {
			return false, testOverrides{ignoreError: true}
		},
		"fail - not found ": func(*testing.T) (bool, testOverrides) {
			return false, testOverrides{
				getClientSetWrapper: func() (kubernetes.Interface, error) {
					fakeClientSet := fake.NewSimpleClientset()
					fakeDiscovery, ok := fakeClientSet.Discovery().(*discoveryfake.FakeDiscovery)
					if !ok {
						t.Fatalf("couldn't convert Discovery() to *FakeDiscovery")
					}
					fakeDiscovery.Resources = []*metav1.APIResourceList{
						{
							APIResources: []metav1.APIResource{
								{Name: "security.k8s.io"},
							},
							GroupVersion: "security.k8s.io/v1",
						},
					}
					return fakeClientSet, nil
				},
			}
		},
		"fail- bad version ": func(*testing.T) (bool, testOverrides) {
			return false, testOverrides{
				getClientSetWrapper: func() (kubernetes.Interface, error) {
					fakeClientSet := fake.NewSimpleClientset()
					fakeDiscovery, ok := fakeClientSet.Discovery().(*discoveryfake.FakeDiscovery)
					if !ok {
						t.Fatalf("couldn't convert Discovery() to *FakeDiscovery")
					}
					fakeDiscovery.Resources = []*metav1.APIResourceList{
						{
							APIResources: []metav1.APIResource{
								{Name: "security.openshift.io"},
							},
							GroupVersion: "security.openshift.io////v1",
						},
					}
					return fakeClientSet, nil
				},
			}
		},
		"fail - to get client set": func(*testing.T) (bool, testOverrides) {
			return false, testOverrides{
				getClientSetWrapper: func() (kubernetes.Interface, error) {
					return fake.NewSimpleClientset(), errors.New(" error listing pods")
				},
			}

		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, patch := tc(t)

			if patch.getClientSetWrapper != nil {
				oldGetClientSetWrapper := GetClientSetWrapper
				defer func() { GetClientSetWrapper = oldGetClientSetWrapper }()
				GetClientSetWrapper = patch.getClientSetWrapper
			}

			isOpenshift, err := IsOpenShift()
			if patch.ignoreError {
				t.Log("cover  real Openshift setup")
			} else if !success {
				assert.False(t, isOpenshift)
			} else {
				assert.NoError(t, err)
				assert.True(t, isOpenshift)
			}

		})
	}
}

func Test_GetVersion(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, string, string, testOverrides){
		"success ": func(*testing.T) (bool, string, string, testOverrides) {
			major := "2"
			minor := "9"
			return true, major, minor, testOverrides{
				getClientSetWrapper: func() (kubernetes.Interface, error) {
					fakeClientSet := fake.NewSimpleClientset()
					fakeClientSet.Discovery().(*discoveryfake.FakeDiscovery).FakedServerVersion = &version.Info{
						Major: major,
						Minor: minor,
					}
					return fakeClientSet, nil
				},
			}

		},
		"fail - to get client set": func(*testing.T) (bool, string, string, testOverrides) {
			return false, "", "", testOverrides{
				getClientSetWrapper: func() (kubernetes.Interface, error) {
					return fake.NewSimpleClientset(), errors.New(" error listing pods")
				},
			}
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, major, minor, patch := tc(t)

			if patch.getClientSetWrapper != nil {
				oldGetClientSetWrapper := GetClientSetWrapper
				defer func() { GetClientSetWrapper = oldGetClientSetWrapper }()
				GetClientSetWrapper = patch.getClientSetWrapper
			}

			out, err := GetKubeAPIServerVersion()
			if !success {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, out, fmt.Sprintf("%s.%s", major, minor))
			}

		})
	}
}

func Test_ControllerRuntimeClient(t *testing.T) {
	tests := map[string]func(t *testing.T) (bool, string){
		"fail - full run": func(*testing.T) (bool, string) {
			content := `
apiVersion: v1
clusters:
- cluster:
    server: https://localhost:8080
    extensions:
    - name: client.authentication.k8s.io/exec
      extension:
        audience: foo
        other: bar
  name: foo-cluster
contexts:
- context:
    cluster: foo-cluster
    user: foo-user
    namespace: bar
  name: foo-context
current-context: foo-context
kind: Config
users:
- name: foo-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1alpha1
      args:
      - arg-1
      - arg-2
      command: foo-command
      provideClusterInfo: true
`
			return false, content
		},
		"fail - RESTConfigFromKubeConfig": func(*testing.T) (bool, string) {
			content := `
apiVersion: v1
clusters:
- cluster:
	certificate-authority-data: ZGF0YS1oZXJl
	server: https://127.0.0.1:6443
	name: kubernetes
contexts:
- context:
	cluster: kubernetes
	user: kubernetes-admin
	name: kubernetes-admin@kubernetes
current-context: kubernetes-admin@kubernetes
kind: Config
preferences: {}
users:
- name: kubernetes-admin
	user:
	client-certificate-data: ZGF0YS1oZXJl
	client-key-data: ZGF0YS1oZXJl
`
			return false, content
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			success, data := tc(t)

			_, err := NewControllerRuntimeClient([]byte(data))

			if !success {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

		})
	}
}
