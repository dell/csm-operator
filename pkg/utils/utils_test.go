package utils_test

import (
	"os"
	"testing"

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
