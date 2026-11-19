// ****
// Secure code generated utilizing Code Guardian.
// Applicable rules used: CG-K8S-001.2, CG-K8S-001.7
// ****

// Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package modules

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	csmv1 "github.com/dell/csm-operator/api/v1"
)

// legacyRedisCommanderDeployment reproduces the Deployment shape created by
// operator versions that still deployed Redis Commander: the csm label exists
// only on the pod template, never on the Deployment metadata.
func legacyRedisCommanderDeployment(crName, name, namespace string) appsv1.Deployment {
	var replicas int32 = 1
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"csm":  crName,
						"app":  name,
						"tier": "backend",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  name,
						Image: "rediscommander:latest",
						Env: []corev1.EnvVar{
							{Name: "HTTP_PASSWORD"},
							{Name: "HTTP_USER"},
						},
						Ports: []corev1.ContainerPort{{ContainerPort: 8081, Name: name}},
					}},
				},
			},
		},
	}
}

func TestDeleteAuthorizationRedisCommanderDeploymentsDiscoversLegacyTemplateLabels(t *testing.T) {
	const (
		crName    = "test-cr"
		name      = "legacy-redis-ui"
		namespace = "default"
	)

	deployment := legacyRedisCommanderDeployment(crName, name, namespace)
	if _, ok := deployment.Labels["csm"]; ok {
		t.Fatal("legacy fixture must not put the csm label on Deployment metadata")
	}
	if deployment.Spec.Template.Labels["csm"] != crName {
		t.Fatalf("expected legacy fixture pod template csm label %q, got %q", crName, deployment.Spec.Template.Labels["csm"])
	}

	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}}
	ctrlClient := fake.NewClientBuilder().WithObjects(&deployment, service).Build()

	if err := deleteAuthorizationRedisCommanderDeployments(
		context.Background(),
		crName,
		namespace,
		csmv1.Module{},
		ctrlClient,
	); err != nil {
		t.Fatalf("deleteAuthorizationRedisCommanderDeployments() returned error: %v", err)
	}

	err := ctrlClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, &appsv1.Deployment{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected legacy Redis Commander deployment to be deleted, got error: %v", err)
	}
	err = ctrlClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, &corev1.Service{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected legacy Redis Commander service to be deleted, got error: %v", err)
	}
}

func TestDeleteAuthorizationRedisCommanderDeploymentsDoesNotDeleteAnotherCSM(t *testing.T) {
	const (
		crName    = "test-cr"
		name      = "other-redis-ui"
		namespace = "default"
	)

	deployment := legacyRedisCommanderDeployment("other-cr", name, namespace)
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}}
	ctrlClient := fake.NewClientBuilder().WithObjects(&deployment, service).Build()

	if err := deleteAuthorizationRedisCommanderDeployments(
		context.Background(),
		crName,
		namespace,
		csmv1.Module{},
		ctrlClient,
	); err != nil {
		t.Fatalf("deleteAuthorizationRedisCommanderDeployments() returned error: %v", err)
	}

	if err := ctrlClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, &appsv1.Deployment{}); err != nil {
		t.Fatalf("expected another CSM's Redis Commander deployment to remain, got error: %v", err)
	}
	if err := ctrlClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, &corev1.Service{}); err != nil {
		t.Fatalf("expected another CSM's Redis Commander service to remain, got error: %v", err)
	}
}
