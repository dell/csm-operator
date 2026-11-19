// ****
// Secure code generated utilizing Code Guardian.
// Applicable rules used: CG-DOCKER-001.6, CG-DOCKER-001.2, CG-INFRA-001.2, CG-INFRA-001.3, CG-INFRA-001.8
// ****

/*
 * Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

func TestDeleteAuthorizationRedisCommanderDeployments(t *testing.T) {
	ctx := context.Background()
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-rediscommander",
			Namespace: "default",
		},
	}
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
		},
	}
	ctrlClient := fake.NewClientBuilder().WithObjects(deployment, service).Build()
	authModule := csmv1.Module{
		Components: []csmv1.ContainerTemplate{
			{
				Name:           AuthRedisComponent,
				RedisCommander: "custom-rediscommander",
			},
		},
	}

	if err := deleteAuthorizationRedisCommanderDeployments(ctx, "test-cr", "default", authModule, ctrlClient); err != nil {
		t.Fatalf("deleteAuthorizationRedisCommanderDeployments() returned error: %v", err)
	}

	err := ctrlClient.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, &appsv1.Deployment{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected Redis Commander deployment to be deleted, got error: %v", err)
	}
	err = ctrlClient.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, &corev1.Service{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected Redis Commander service to be deleted, got error: %v", err)
	}
}

func TestDeleteAuthorizationRedisCommanderDeploymentsFindsLegacyCustomName(t *testing.T) {
	ctx := context.Background()
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "legacy-redis-ui",
			Namespace: "default",
			Labels: map[string]string{
				"csm": "test-cr",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "legacy-redis-ui",
						Env: []corev1.EnvVar{
							{Name: "HTTP_USER"},
							{Name: "HTTP_PASSWORD"},
						},
						Ports: []corev1.ContainerPort{{ContainerPort: 8081}},
					}},
				},
			},
		},
	}
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      deployment.Name,
		Namespace: deployment.Namespace,
	}}
	ctrlClient := fake.NewClientBuilder().WithObjects(deployment, service).Build()

	if err := deleteAuthorizationRedisCommanderDeployments(
		ctx,
		"test-cr",
		"default",
		csmv1.Module{},
		ctrlClient,
	); err != nil {
		t.Fatalf("deleteAuthorizationRedisCommanderDeployments() returned error: %v", err)
	}

	err := ctrlClient.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, &appsv1.Deployment{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected legacy Redis Commander deployment to be deleted, got error: %v", err)
	}
	err = ctrlClient.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, &corev1.Service{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected legacy Redis Commander service to be deleted, got error: %v", err)
	}
}

func TestValidateRedisConfigSecretProviderWithoutUsernameKey(t *testing.T) {
	ok, err := validateRedisConfig(csmv1.ContainerTemplate{
		Name: AuthRedisComponent,
		RedisSecretProviderClass: []csmv1.RedisSecretProviderClass{{
			SecretProviderClassName: "redis-spc",
			RedisSecretName:         "redis-secret",
			RedisPasswordKey:        "password",
		}},
	})
	if err != nil {
		t.Fatalf("validateRedisConfig() returned error: %v", err)
	}
	if ok {
		t.Fatal("expected secret-provider credentials to not be classified as direct credentials")
	}
}

func TestCreateRedisK8sSecretWithoutUsername(t *testing.T) {
	secret := createRedisK8sSecret("redis-csm-secret", "default", "", "test-password")
	if _, ok := secret.StringData["commander_user"]; ok {
		t.Fatal("expected Commander-only username key to be omitted")
	}
	if secret.StringData["password"] != "test-password" {
		t.Fatalf("expected password to be preserved, got %q", secret.StringData["password"])
	}
}

func TestRedisConjurPathsPasswordOnly(t *testing.T) {
	paths := redisConjurPaths("", "secrets/redis-password")
	if len(paths) != 1 || paths[0] != "secrets/redis-password" {
		t.Fatalf("expected only the password path, got %v", paths)
	}
}

func TestValidateRedisConfigConjurRequiresPasswordPath(t *testing.T) {
	_, err := validateRedisConfig(csmv1.ContainerTemplate{
		Name: AuthRedisComponent,
		RedisSecretProviderClass: []csmv1.RedisSecretProviderClass{{
			SecretProviderClassName: "redis-spc",
			RedisSecretName:         "redis-secret",
			RedisPasswordKey:        "password",
			Conjur: &csmv1.ConjurCredentialPath{
				UsernamePath: "secrets/redis-username",
			},
		}},
	})
	if err == nil {
		t.Fatal("expected Conjur configuration without passwordPath to be rejected")
	}
}

func TestValidateRedisConfigConjurPasswordOnly(t *testing.T) {
	if _, err := validateRedisConfig(csmv1.ContainerTemplate{
		Name: AuthRedisComponent,
		RedisSecretProviderClass: []csmv1.RedisSecretProviderClass{{
			SecretProviderClassName: "redis-spc",
			RedisSecretName:         "redis-secret",
			RedisPasswordKey:        "password",
			Conjur: &csmv1.ConjurCredentialPath{
				PasswordPath: "secrets/redis-password",
			},
		}},
	}); err != nil {
		t.Fatalf("expected password-only Conjur configuration to be accepted: %v", err)
	}
}
