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

			out, err := GetVersion()
			if !success {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, out, fmt.Sprintf("%s.%s", major, minor))
			}

		})
	}
}
