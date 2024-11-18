//  Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package daemonset

import (
	"context"
	"fmt"

	// "errors"
	"testing"

	// "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	confcorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	v1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"
)

// Mock logger
func getTestLogger() *zap.SugaredLogger {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logger, _ := config.Build()
	return logger.Sugar()
}

func TestSyncDaemonset(t *testing.T) {
	ctx := context.Background()
	logger := getTestLogger()
	defer func() {
		_ = logger.Sync() // ignore the error from logger.Sync()
	}()

	daemonset := appsv1.DaemonSetApplyConfiguration{
		ObjectMetaApplyConfiguration: &v1.ObjectMetaApplyConfiguration{
			Name:      stringPtr("test-daemonset"),
			Namespace: stringPtr("test-namespace"),
		},
		Spec: &appsv1.DaemonSetSpecApplyConfiguration{
			Template: &confcorev1.PodTemplateSpecApplyConfiguration{
				ObjectMetaApplyConfiguration: &v1.ObjectMetaApplyConfiguration{
					Labels: map[string]string{},
				},
			},
		},
	}

	t.Run("Daemonset labels are initialized", func(t *testing.T) {
		k8sClient := fake.NewSimpleClientset(&apps.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-daemonset",
				Namespace: "test-namespace",
			},
			Spec: apps.DaemonSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image",
							},
						},
					},
				},
			},
		})

		err := SyncDaemonset(ctx, daemonset, k8sClient, "test-csm")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if daemonset.Spec.Template.Labels["csm"] != "test-csm" {
			t.Fatalf("expected label 'csm' to be 'test-csm', got %v", daemonset.Spec.Template.Labels["csm"])
		}
	})

	t.Run("Handle error on getting DaemonSet", func(t *testing.T) {
		k8sClient := fake.NewSimpleClientset()

		// error getting daemonset
		k8sClient.PrependReactor("get", "daemonsets", func(_ k8sTesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, apierrors.NewInternalError(fmt.Errorf("internal error"))
		})

		err := SyncDaemonset(ctx, daemonset, k8sClient, "test-csm")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("DaemonSet not found", func(t *testing.T) {
		k8sClient := fake.NewSimpleClientset()

		// daemonset not found
		k8sClient.PrependReactor("get", "daemonsets", func(_ k8sTesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "daemonsets"}, "test-daemonset")
		})

		err := SyncDaemonset(ctx, daemonset, k8sClient, "test-csm")
		assert.Error(t, err)
	})

	t.Run("Handle empty labels", func(t *testing.T) {
		// daemonset with nil Labels
		daemonset := appsv1.DaemonSetApplyConfiguration{
			ObjectMetaApplyConfiguration: &v1.ObjectMetaApplyConfiguration{
				Name:      stringPtr("test-daemonset"),
				Namespace: stringPtr("default"),
			},
			Spec: &appsv1.DaemonSetSpecApplyConfiguration{
				Template: &confcorev1.PodTemplateSpecApplyConfiguration{
					ObjectMetaApplyConfiguration: &v1.ObjectMetaApplyConfiguration{
						Labels: nil,
					},
				},
			},
		}

		k8sClient := fake.NewSimpleClientset(&apps.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-daemonset",
				Namespace: "default",
			},
			Spec: apps.DaemonSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image",
							},
						},
					},
				},
			},
		})

		err := SyncDaemonset(ctx, daemonset, k8sClient, "test-csm")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if daemonset.Spec.Template.Labels["csm"] != "test-csm" {
			t.Fatalf("expected label 'csm' to be 'test-csm', got %v", daemonset.Spec.Template.Labels["csm"])
		}
	})
}

func stringPtr(s string) *string {
	return &s
}
