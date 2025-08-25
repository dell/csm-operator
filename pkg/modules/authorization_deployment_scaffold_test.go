// Copyright (c) 2025 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package modules

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestGetProxyServerScaffold(t *testing.T) {
	name := "test-csm"
	sentinelName := "sentinel"
	namespace := "test-namespace"
	proxyImage := "proxy-image:latest"
	opaImage := "opa-image:latest"
	opaKubeMgmtImage := "kube-mgmt-image:latest"
	redisSecretName := "redis-secret"
	redisPasswordKey := "redis-password"
	replicas := int32(3)
	sentinel := int(5)

	deploy := getProxyServerScaffold(name, sentinelName, namespace, proxyImage, opaImage, opaKubeMgmtImage, redisSecretName, redisPasswordKey, replicas, sentinel)

	if deploy.Name != "proxy-server" {
		t.Errorf("expected name 'proxy-server', got %s", deploy.Name)
	}
	if deploy.Namespace != namespace {
		t.Errorf("expected namespace %s, got %s", namespace, deploy.Namespace)
	}
	if *deploy.Spec.Replicas != replicas {
		t.Errorf("expected replicas %d, got %d", replicas, *deploy.Spec.Replicas)
	}

	if len(deploy.Spec.Template.Spec.Containers) != 3 {
		t.Errorf("expected 3 containers, got %d", len(deploy.Spec.Template.Spec.Containers))
	}

	envVars := deploy.Spec.Template.Spec.Containers[0].Env
	found := false
	for _, env := range envVars {
		if env.Name == "REDIS_PASSWORD" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected REDIS_PASSWORD env var not found")
	}
}

func TestGetStorageServiceScaffold(t *testing.T) {
	name := "test-csm"
	namespace := "test-namespace"
	image := "storage-service:latest"
	replicas := int32(2)

	deploy := getStorageServiceScaffold(name, namespace, image, replicas)

	if deploy.Name != "storage-service" {
		t.Errorf("expected name 'storage-service', got %s", deploy.Name)
	}
	if deploy.Namespace != namespace {
		t.Errorf("expected namespace %s, got %s", namespace, deploy.Namespace)
	}
	if *deploy.Spec.Replicas != replicas {
		t.Errorf("expected replicas %d, got %d", replicas, *deploy.Spec.Replicas)
	}

	// Labels
	labels := deploy.Spec.Template.Labels
	if labels["csm"] != name {
		t.Errorf("expected label csm=%s, got %s", name, labels["csm"])
	}

	// Container checks
	containers := deploy.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	container := containers[0]
	if container.Name != "storage-service" {
		t.Errorf("expected container name 'storage-service', got %s", container.Name)
	}
	if container.Image != image {
		t.Errorf("expected image %s, got %s", image, container.Image)
	}

	// Environment variable
	foundEnv := false
	for _, env := range container.Env {
		if env.Name == "NAMESPACE" && env.Value == namespace {
			foundEnv = true
			break
		}
	}
	if !foundEnv {
		t.Error("expected NAMESPACE env var with correct value not found")
	}

	// Volume mounts
	expectedMounts := map[string]string{
		"config-volume":     "/etc/karavi-authorization/config",
		"csm-config-params": "/etc/karavi-authorization/csm-config-params",
	}
	for _, mount := range container.VolumeMounts {
		expectedPath, ok := expectedMounts[mount.Name]
		if !ok {
			t.Errorf("unexpected volume mount: %s", mount.Name)
		} else if mount.MountPath != expectedPath {
			t.Errorf("expected mount path %s for volume %s, got %s", expectedPath, mount.Name, mount.MountPath)
		}
	}
}

func TestGetTenantServiceScaffold(t *testing.T) {
	name := "test-csm"
	namespace := "test-namespace"
	sentinelName := "sentinel"
	image := "tenant-service:latest"
	redisSecretName := "redis-secret"
	redisPasswordKey := "redis-password"
	replicas := int32(3)
	sentinelReplicas := 5

	deploy := getTenantServiceScaffold(name, namespace, sentinelName, image, redisSecretName, redisPasswordKey, replicas, sentinelReplicas)

	if deploy.Name != "tenant-service" {
		t.Errorf("expected name 'tenant-service', got %s", deploy.Name)
	}
	if deploy.Namespace != namespace {
		t.Errorf("expected namespace %s, got %s", namespace, deploy.Namespace)
	}
	if *deploy.Spec.Replicas != replicas {
		t.Errorf("expected replicas %d, got %d", replicas, *deploy.Spec.Replicas)
	}

	// Labels
	labels := deploy.Spec.Template.Labels
	if labels["csm"] != name {
		t.Errorf("expected label csm=%s, got %s", name, labels["csm"])
	}

	// Container
	containers := deploy.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	container := containers[0]
	if container.Name != "tenant-service" {
		t.Errorf("expected container name 'tenant-service', got %s", container.Name)
	}
	if container.Image != image {
		t.Errorf("expected image %s, got %s", image, container.Image)
	}

	// Env vars
	var foundRedisPassword bool
	for _, env := range container.Env {
		if env.Name == "REDIS_PASSWORD" && env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			if env.ValueFrom.SecretKeyRef.Key == redisPasswordKey {
				foundRedisPassword = true
			}
		}
	}
	if !foundRedisPassword {
		t.Error("expected REDIS_PASSWORD env var with correct secret key not found")
	}

	// Args
	expectedArgs := []string{
		"--redis-sentinel=$(SENTINELS)",
		"--redis-password=$(REDIS_PASSWORD)",
	}
	if len(container.Args) != len(expectedArgs) {
		t.Errorf("expected %d args, got %d", len(expectedArgs), len(container.Args))
	}
	for i, arg := range expectedArgs {
		if container.Args[i] != arg {
			t.Errorf("expected arg %d to be %s, got %s", i, arg, container.Args[i])
		}
	}

	// Ports
	if len(container.Ports) != 1 || container.Ports[0].ContainerPort != 50051 {
		t.Error("expected container port 50051 not found")
	}

	// Volume mounts
	expectedMounts := map[string]string{
		"config-volume":     "/etc/karavi-authorization/config",
		"csm-config-params": "/etc/karavi-authorization/csm-config-params",
	}
	for _, mount := range container.VolumeMounts {
		expectedPath, ok := expectedMounts[mount.Name]
		if !ok {
			t.Errorf("unexpected volume mount: %s", mount.Name)
		} else if mount.MountPath != expectedPath {
			t.Errorf("expected mount path %s for volume %s, got %s", expectedPath, mount.Name, mount.MountPath)
		}
	}
}

func TestGetAuthorizationRedisStatefulsetScaffold(t *testing.T) {
	crName := "test-cr"
	name := "redis"
	namespace := "default"
	image := "redis:latest"
	redisSecretName := "redis-secret"
	redisPasswordKey := "password"
	replicas := int32(3)
	hash := sha256.Sum256([]byte("data"))
	checksum := hex.EncodeToString(hash[:])

	sts := getAuthorizationRedisStatefulsetScaffold(crName, name, namespace, image, redisSecretName, redisPasswordKey, checksum, replicas)

	if sts.Name != name {
		t.Errorf("expected name %s, got %s", name, sts.Name)
	}
	if sts.Namespace != namespace {
		t.Errorf("expected namespace %s, got %s", namespace, sts.Namespace)
	}
	if *sts.Spec.Replicas != replicas {
		t.Errorf("expected replicas %d, got %d", replicas, *sts.Spec.Replicas)
	}
	if sts.Spec.Template.Labels["csm"] != crName {
		t.Errorf("expected label csm=%s, got %s", crName, sts.Spec.Template.Labels["csm"])
	}
	if sts.Spec.Template.Spec.Containers[0].Image != image {
		t.Errorf("expected container image %s, got %s", image, sts.Spec.Template.Spec.Containers[0].Image)
	}
	if sts.Spec.Template.Spec.InitContainers[0].Env[0].ValueFrom.SecretKeyRef.Name != redisSecretName {
		t.Errorf("expected secret name %s, got %s", redisSecretName, sts.Spec.Template.Spec.InitContainers[0].Env[0].ValueFrom.SecretKeyRef.Name)
	}
	if sts.Spec.Template.Spec.InitContainers[0].Env[0].ValueFrom.SecretKeyRef.Key != redisPasswordKey {
		t.Errorf("expected secret key %s, got %s", redisPasswordKey, sts.Spec.Template.Spec.InitContainers[0].Env[0].ValueFrom.SecretKeyRef.Key)
	}
}

func TestGetAuthorizationRediscommanderDeploymentScaffold(t *testing.T) {
	crName := "test-cr"
	name := "redis-commander"
	sentinelName := "sentinel"
	namespace := "default"
	image := "rediscommander:latest"
	redisSecretName := "redis-secret"
	redisUsernameKey := "username"
	redisPasswordKey := "password"
	replicas := 5
	hash := sha256.Sum256([]byte("data"))
	checksum := hex.EncodeToString(hash[:])

	deploy := getAuthorizationRediscommanderDeploymentScaffold(crName, name, namespace, image, redisSecretName, redisUsernameKey, redisPasswordKey, sentinelName, checksum, replicas)

	envVars := deploy.Spec.Template.Spec.Containers[0].Env
	found := false
	for _, env := range envVars {
		if env.Name == "SENTINELS" && env.Value == "sentinel-0.sentinel.default.svc.cluster.local:5000,sentinel-1.sentinel.default.svc.cluster.local:5000,sentinel-2.sentinel.default.svc.cluster.local:5000,sentinel-3.sentinel.default.svc.cluster.local:5000,sentinel-4.sentinel.default.svc.cluster.local:5000" {
			found = true
			break
		}
	}
	fmt.Println(envVars)
	if !found {
		t.Errorf("expected mocked SENTINELS env var, but not found")
	}
}

func TestGetAuthorizationSentinelStatefulsetScaffold(t *testing.T) {
	crName := "test-cr"
	name := "sentinel"
	redisName := "redis-csm"
	namespace := "default"
	image := "redis:7.0"
	redisSecretName := "redis-secret"
	redisPasswordKey := "password"
	replicas := int32(3)
	hash := sha256.Sum256([]byte("data"))
	checksum := hex.EncodeToString(hash[:])

	sts := getAuthorizationSentinelStatefulsetScaffold(crName, name, redisName, namespace, image, redisSecretName, redisPasswordKey, checksum, replicas)

	if sts.Name != name {
		t.Errorf("expected name %s, got %s", name, sts.Name)
	}
	if *sts.Spec.Replicas != replicas {
		t.Errorf("expected replicas %d, got %d", replicas, *sts.Spec.Replicas)
	}
	if sts.Spec.ServiceName != name {
		t.Errorf("expected service name %s, got %s", name, sts.Spec.ServiceName)
	}

	// Label checks
	labels := sts.Spec.Template.Labels
	if labels["csm"] != crName || labels["app"] != name {
		t.Errorf("unexpected labels: %+v", labels)
	}

	// Init container checks
	initContainer := sts.Spec.Template.Spec.InitContainers[0]
	if initContainer.Name != "config" {
		t.Errorf("expected init container name 'config', got %s", initContainer.Name)
	}
	if initContainer.Image != image {
		t.Errorf("expected init container image %s, got %s", image, initContainer.Image)
	}
	if len(initContainer.Env) == 0 || initContainer.Env[0].Name != "REDIS_PASSWORD" {
		t.Errorf("expected REDIS_PASSWORD env var in init container")
	}
	if initContainer.Env[0].ValueFrom.SecretKeyRef.Name != redisSecretName {
		t.Errorf("expected secret name %s, got %s", redisSecretName, initContainer.Env[0].ValueFrom.SecretKeyRef.Name)
	}
	if initContainer.Env[0].ValueFrom.SecretKeyRef.Key != redisPasswordKey {
		t.Errorf("expected secret key %s, got %s", redisPasswordKey, initContainer.Env[0].ValueFrom.SecretKeyRef.Key)
	}

	// Main container checks
	mainContainer := sts.Spec.Template.Spec.Containers[0]
	if mainContainer.Name != name {
		t.Errorf("expected main container name %s, got %s", name, mainContainer.Name)
	}
	if mainContainer.Image != image {
		t.Errorf("expected main container image %s, got %s", image, mainContainer.Image)
	}
	if len(mainContainer.Ports) == 0 || mainContainer.Ports[0].ContainerPort != 5000 {
		t.Errorf("expected container port 5000, got %+v", mainContainer.Ports)
	}

	// Volume checks
	volumeNames := map[string]bool{}
	for _, vol := range sts.Spec.Template.Spec.Volumes {
		volumeNames[vol.Name] = true
	}
	if !volumeNames["redis-config"] || !volumeNames["data"] {
		t.Errorf("expected volumes 'redis-config' and 'data' to be present, got %+v", volumeNames)
	}
}

func TestCreateRedisK8sSecret(t *testing.T) {
	secret := createRedisK8sSecret("name", "test-namespace")

	if secret.Name != defaultRedisSecretName {
		t.Errorf("expected secret name %s, got %s", defaultRedisSecretName, secret.Name)
	}
	if secret.Namespace != "test-namespace" {
		t.Errorf("expected namespace 'test-namespace', got %s", secret.Namespace)
	}
	if secret.Type != corev1.SecretTypeBasicAuth {
		t.Errorf("expected secret type BasicAuth, got %s", secret.Type)
	}
	if secret.StringData["username"] != "dev" {
		t.Errorf("expected username 'dev', got %s", secret.StringData["username"])
	}
	if secret.StringData["password"] != "K@ravi123!" {
		t.Errorf("expected password 'K@ravi123!', got %s", secret.StringData["password"])
	}
}

func TestRedisVolume(t *testing.T) {
	secretName := "redis-secret"
	volume := redisVolume(secretName)

	if volume.Name != "secrets-store-inline-redis" {
		t.Errorf("expected volume name 'secrets-store-inline-redis', got %s", volume.Name)
	}
	if volume.VolumeSource.CSI == nil {
		t.Fatal("expected CSI volume source, got nil")
	}
	if volume.VolumeSource.CSI.Driver != "secrets-store.csi.k8s.io" {
		t.Errorf("expected CSI driver 'secrets-store.csi.k8s.io', got %s", volume.VolumeSource.CSI.Driver)
	}
	if volume.VolumeSource.CSI.VolumeAttributes["secretProviderClass"] != secretName {
		t.Errorf("expected secretProviderClass '%s', got %s", secretName, volume.VolumeSource.CSI.VolumeAttributes["secretProviderClass"])
	}
}

func TestRedisVolumeMount(t *testing.T) {
	mount := redisVolumeMount()

	if mount.Name != "secrets-store-inline-redis" {
		t.Errorf("expected mount name 'secrets-store-inline-redis', got %s", mount.Name)
	}
	if mount.MountPath != "/etc/csm-authorization/redis" {
		t.Errorf("expected mount path '/etc/csm-authorization/redis', got %s", mount.MountPath)
	}
	if !mount.ReadOnly {
		t.Error("expected mount to be read-only")
	}
}
