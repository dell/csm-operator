package utils_test

import (
	"os"
	"testing"

	csmv1 "github.com/dell/csm-operator/api/v1"
	"github.com/dell/csm-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetDriverYAML(t *testing.T) {
	type checkFn func(*testing.T, interface{}, error)
	check := func(fns ...checkFn) []checkFn { return fns }

	hasNoError := func(t *testing.T, result interface{}, err error) {
		if err != nil {
			t.Fatalf("expected no error but found %v", err)
		}
	}

	checkExpectedOutput := func(expectedOutput interface{}) func(t *testing.T, result interface{}, err error) {
		return func(t *testing.T, result interface{}, err error) {
			assert.Equal(t, expectedOutput, result)
		}
	}

	hasError := func(t *testing.T, result interface{}, err error) {
		if err == nil {
			t.Fatalf("expected error")
		}
	}

	tests := map[string]func(t *testing.T) (string, string, []checkFn){

		"deployment": func(*testing.T) (string, string, []checkFn) {
			dat, err := os.ReadFile("test-data/deployment.yaml")
			if err != nil {
				t.Fatal(err)
			}
			d := utils.ControllerYAML{
				Deployment: appsv1.Deployment{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Deployment",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "deployment",
					},
				},
				Rbac: utils.RbacYAML{
					ServiceAccount: corev1.ServiceAccount{
						TypeMeta: metav1.TypeMeta{
							Kind:       "ServiceAccount",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "service-account",
						},
					},
					ClusterRole: rbacv1.ClusterRole{
						TypeMeta: metav1.TypeMeta{
							Kind:       "ClusterRole",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster-role",
						},
					},
					ClusterRoleBinding: rbacv1.ClusterRoleBinding{
						TypeMeta: metav1.TypeMeta{
							Kind:       "ClusterRoleBinding",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster-role-binding",
						},
					},
				},
			}
			return string(dat), "Deployment", check(hasNoError, checkExpectedOutput(d))
		},
		"daemonset": func(*testing.T) (string, string, []checkFn) {
			dat, err := os.ReadFile("test-data/daemon-set.yaml")
			if err != nil {
				t.Fatal(err)
			}
			d := utils.NodeYAML{
				DaemonSet: appsv1.DaemonSet{
					TypeMeta: metav1.TypeMeta{
						Kind:       "DaemonSet",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "daemon-set",
					},
				},
				Rbac: utils.RbacYAML{
					ServiceAccount: corev1.ServiceAccount{
						TypeMeta: metav1.TypeMeta{
							Kind:       "ServiceAccount",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "service-account",
						},
					},
					ClusterRole: rbacv1.ClusterRole{
						TypeMeta: metav1.TypeMeta{
							Kind:       "ClusterRole",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster-role",
						},
					},
					ClusterRoleBinding: rbacv1.ClusterRoleBinding{
						TypeMeta: metav1.TypeMeta{
							Kind:       "ClusterRoleBinding",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster-role-binding",
						},
					},
				},
			}
			return string(dat), "DaemonSet", check(hasNoError, checkExpectedOutput(d))
		},
		"invalid kind": func(*testing.T) (string, string, []checkFn) {
			return "", "not-a-kind", check(hasError)
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			yaml, kind, checkFns := tc(t)
			result, err := utils.GetDriverYAML(yaml, kind)
			for _, checkFn := range checkFns {
				checkFn(t, result, err)
			}
		})
	}
}

func Test_UpdateSideCar(t *testing.T) {
	type checkFn func(*testing.T, corev1.Container)
	check := func(fns ...checkFn) []checkFn { return fns }

	checkExpectedOutput := func(expectedOutput corev1.Container) func(t *testing.T, c corev1.Container) {
		return func(t *testing.T, result corev1.Container) {
			assert.Equal(t, expectedOutput, result)
		}
	}

	tests := map[string]func(t *testing.T) ([]csmv1.ContainerTemplate, corev1.Container, []checkFn){

		"deployment": func(*testing.T) ([]csmv1.ContainerTemplate, corev1.Container, []checkFn) {

			sidecars := []csmv1.ContainerTemplate{
				{
					Name:            "my-sidecar-1",
					Image:           "my-image-1",
					ImagePullPolicy: "my-pull-policy-1",
				},
				{
					Name:            "my-sidecar-2",
					Image:           "my-image-2",
					ImagePullPolicy: "my-pull-policy-2",
					Envs: []corev1.EnvVar{
						{
							Name:  "name-1",
							Value: "value-1",
						},
						{
							Name:  "name-2",
							Value: "value-2",
						},
					},
					Args: []string{"arg1=val1", "arg2=val2"},
				},
				{
					Name:            "my-sidecar-3",
					Image:           "my-image-3",
					ImagePullPolicy: "my-pull-policy-3",
				},
			}

			container := corev1.Container{
				Name: "my-sidecar-2",
				Env: []corev1.EnvVar{
					{
						Name:  "name-2",
						Value: "old-value-2",
					},
				},
				Args: []string{"arg1=oldval1"},
			}

			expectedResult := corev1.Container{
				Name:            "my-sidecar-2",
				Image:           "my-image-2",
				ImagePullPolicy: "my-pull-policy-2",
				Env: []corev1.EnvVar{
					{
						Name:  "name-2",
						Value: "value-2",
					},
					{
						Name:  "name-1",
						Value: "value-1",
					},
				},
				Args: []string{"arg1=val1", "arg2=val2"},
			}
			return sidecars, container, check(checkExpectedOutput(expectedResult))
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			sidecars, container, checkFns := tc(t)
			result := utils.UpdateSideCar(sidecars, container)
			for _, checkFn := range checkFns {
				checkFn(t, result)
			}
		})
	}
}

func Test_ReplaceALLContainerImage(t *testing.T) {
	type checkFn func(*testing.T, corev1.Container)
	check := func(fns ...checkFn) []checkFn { return fns }

	checkExpectedOutput := func(expectedOutput corev1.Container) func(t *testing.T, c corev1.Container) {
		return func(t *testing.T, result corev1.Container) {
			assert.Equal(t, expectedOutput, result)
		}
	}

	img := utils.K8sImagesConfig{}
	img.Images.Provisioner = "provisioner-image"
	img.Images.Attacher = "attacher-image"
	img.Images.Registrar = "registrar-image"
	img.Images.Resizer = "resizer-image"
	img.Images.Snapshotter = "snapshotter-image"

	tests := map[string]func(t *testing.T) (utils.K8sImagesConfig, corev1.Container, []checkFn){

		"provisioner": func(*testing.T) (utils.K8sImagesConfig, corev1.Container, []checkFn) {

			container := corev1.Container{
				Name: csmv1.Provisioner,
			}

			expectedResult := corev1.Container{
				Name:  csmv1.Provisioner,
				Image: "provisioner-image",
			}
			return img, container, check(checkExpectedOutput(expectedResult))
		},
		"attacher": func(*testing.T) (utils.K8sImagesConfig, corev1.Container, []checkFn) {

			container := corev1.Container{
				Name: csmv1.Attacher,
			}

			expectedResult := corev1.Container{
				Name:  csmv1.Attacher,
				Image: "attacher-image",
			}
			return img, container, check(checkExpectedOutput(expectedResult))
		},
		"registrar": func(*testing.T) (utils.K8sImagesConfig, corev1.Container, []checkFn) {

			container := corev1.Container{
				Name: csmv1.Registrar,
			}

			expectedResult := corev1.Container{
				Name:  csmv1.Registrar,
				Image: "registrar-image",
			}
			return img, container, check(checkExpectedOutput(expectedResult))
		},
		"resizer": func(*testing.T) (utils.K8sImagesConfig, corev1.Container, []checkFn) {

			container := corev1.Container{
				Name: csmv1.Resizer,
			}

			expectedResult := corev1.Container{
				Name:  csmv1.Resizer,
				Image: "resizer-image",
			}
			return img, container, check(checkExpectedOutput(expectedResult))
		},
		"snapshotter": func(*testing.T) (utils.K8sImagesConfig, corev1.Container, []checkFn) {

			container := corev1.Container{
				Name: csmv1.Snapshotter,
			}

			expectedResult := corev1.Container{
				Name:  csmv1.Snapshotter,
				Image: "snapshotter-image",
			}
			return img, container, check(checkExpectedOutput(expectedResult))
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			sidecars, container, checkFns := tc(t)
			result := utils.ReplaceALLContainerImage(sidecars, container)
			for _, checkFn := range checkFns {
				checkFn(t, result)
			}
		})
	}
}

func Test_ModifyCommonCR(t *testing.T) {
	type checkFn func(*testing.T, string)
	check := func(fns ...checkFn) []checkFn { return fns }

	checkExpectedOutput := func(expectedOutput string) func(t *testing.T, c string) {
		return func(t *testing.T, result string) {
			assert.Equal(t, expectedOutput, result)
		}
	}

	tests := map[string]func(t *testing.T) (string, csmv1.ContainerStorageModule, []checkFn){

		"success": func(*testing.T) (string, csmv1.ContainerStorageModule, []checkFn) {

			data, err := os.ReadFile("test-data/sample-template-in.yaml")
			if err != nil {
				t.Fatal(err)
			}

			yaml := string(data)

			cr := csmv1.ContainerStorageModule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-csm",
					Namespace: "my-namespace",
				},
				Spec: csmv1.ContainerStorageModuleSpec{
					Driver: csmv1.Driver{
						Common: csmv1.ContainerTemplate{
							ImagePullPolicy: "Always",
						},
					},
				},
			}

			data, err = os.ReadFile("test-data/sample-template-out.yaml")
			if err != nil {
				t.Fatal(err)
			}

			expectedResult := string(data)

			return yaml, cr, check(checkExpectedOutput(expectedResult))
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			sidecars, container, checkFns := tc(t)
			result := utils.ModifyCommonCR(sidecars, container)
			for _, checkFn := range checkFns {
				checkFn(t, result)
			}
		})
	}
}
