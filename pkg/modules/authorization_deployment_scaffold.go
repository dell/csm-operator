// Copyright (c) 2025 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package modules

import (
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// getProxyServerScaffold returns proxy-server deployment for authorization v2
func getProxyServerScaffold(name, sentinelName, namespace, proxyImage, opaImage, opaKubeMgmtImage, configSecretName, redisSecretName, redisPasswordKey string, replicas int32, sentinelReplicas int) appsv1.Deployment {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "csm-config-params",
			MountPath: "/etc/karavi-authorization/csm-config-params",
		},
		{
			Name:      "config-volume",
			MountPath: "/etc/karavi-authorization/config",
		},
	}
	secretName := "karavi-config-secret"
	if configSecretName != "" {
		secretName = configSecretName
	}
	volumes := []corev1.Volume{
		{
			Name: "csm-config-params",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "csm-config-params",
					},
				},
			},
		},
		{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		},
	}

	return appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxy-server",
			Namespace: namespace,
			Labels: map[string]string{
				"app": "proxy-server",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "proxy-server",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"csm": name,
						"app": "proxy-server",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "proxy-server",
					Containers: []corev1.Container{
						{
							Name:            "proxy-server",
							Image:           proxyImage,
							ImagePullPolicy: "Always",
							Env: []corev1.EnvVar{
								{
									Name:  "SENTINELS",
									Value: buildSentinelList(sentinelReplicas, sentinelName, namespace),
								},
								{
									Name: "REDIS_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: redisSecretName,
											},
											Key: redisPasswordKey,
										},
									},
								},
							},
							Args: []string{
								"--redis-sentinel=$(SENTINELS)",
								"--redis-password=$(REDIS_PASSWORD)",
								fmt.Sprintf("--tenant-service=tenant-service.%s.svc.cluster.local:50051", namespace),
								fmt.Sprintf("--role-service=role-service.%s.svc.cluster.local:50051", namespace),
								fmt.Sprintf("--storage-service=storage-service.%s.svc.cluster.local:50051", namespace),
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
								},
							},
							VolumeMounts: volumeMounts,
						},
						{
							Name:            "opa",
							Image:           opaImage,
							ImagePullPolicy: "Always",
							Args: []string{
								"run",
								"--ignore=.",
								"--server",
								"--log-level=debug",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8181,
								},
							},
						},
						{
							Name:            "kube-mgmt",
							Image:           opaKubeMgmtImage,
							ImagePullPolicy: "Always",
							Env: []corev1.EnvVar{
								{
									Name:  "NAMESPACE",
									Value: namespace,
								},
							},
							Args: []string{
								fmt.Sprintf("--namespaces=%s", namespace),
								"--enable-data",
							},
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}

// getStorageServiceScaffold returns the storage-service deployment with the common elements between v1 and v2
// callers must ensure that other elements specific for the version get set in the returned deployment
func getStorageServiceScaffold(name string, namespace string, image string, replicas int32, configSecretName string) appsv1.Deployment {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "csm-config-params",
			MountPath: "/etc/karavi-authorization/csm-config-params",
		},
		{
			Name:      "config-volume",
			MountPath: "/etc/karavi-authorization/config",
		},
	}
	secretName := "karavi-config-secret"
	if configSecretName != "" {
		secretName = configSecretName
	}
	volumes := []corev1.Volume{
		{
			Name: "csm-config-params",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "csm-config-params",
					},
				},
			},
		},
		{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		},
	}

	return appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storage-service",
			Namespace: namespace,
			Labels: map[string]string{
				"app": "storage-service",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "storage-service",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"csm": name,
						"app": "storage-service",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "storage-service",
					Containers: []corev1.Container{
						{
							Name:            "storage-service",
							Image:           image,
							ImagePullPolicy: "Always",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 50051,
									Name:          "grpc",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "NAMESPACE",
									Value: namespace,
								},
							},
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}

// getTenantServiceScaffold returns tenant-service deployment for authorization v2
func getTenantServiceScaffold(name, namespace, sentinelName, image, configSecretName, redisSecretName, redisPasswordKey string, replicas int32, sentinelReplicas int) appsv1.Deployment {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "csm-config-params",
			MountPath: "/etc/karavi-authorization/csm-config-params",
		},
		{
			Name:      "config-volume",
			MountPath: "/etc/karavi-authorization/config",
		},
	}
	secretName := "karavi-config-secret"
	if configSecretName != "" {
		secretName = configSecretName
	}
	volumes := []corev1.Volume{
		{
			Name: "csm-config-params",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "csm-config-params",
					},
				},
			},
		},
		{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		},
	}

	return appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-service",
			Namespace: namespace,
			Labels: map[string]string{
				"app": "tenant-service",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "tenant-service",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"csm": name,
						"app": "tenant-service",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "tenant-service",
					Containers: []corev1.Container{
						{
							Name:            "tenant-service",
							Image:           image,
							ImagePullPolicy: "Always",
							Env: []corev1.EnvVar{
								{
									Name:  "SENTINELS",
									Value: buildSentinelList(sentinelReplicas, sentinelName, namespace),
								},
								{
									Name: "REDIS_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: redisSecretName,
											},
											Key: redisPasswordKey,
										},
									},
								},
							},
							Args: []string{
								"--redis-sentinel=$(SENTINELS)",
								"--redis-password=$(REDIS_PASSWORD)",
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 50051,
									Name:          "grpc",
								},
							},
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}

// getAuthorizationRedisStatefulsetScaffold returns redis statefulset for authorization v2
func getAuthorizationRedisStatefulsetScaffold(crName, name, namespace, image, redisSecretName, redisPasswordKey, checksum string, replicas int32) appsv1.StatefulSet {
	volName := "redis-primary-volume"

	return appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name,
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"csm": crName,
						"app": name,
					},
					Annotations: map[string]string{
						"checksum/secret": checksum,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "redis",
					InitContainers: []corev1.Container{
						{
							Name:            "config",
							Image:           image,
							ImagePullPolicy: "Always",
							Env: []corev1.EnvVar{
								{
									Name: "REDIS_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: redisSecretName,
											},
											Key: redisPasswordKey,
										},
									},
								},
								{
									Name:  "AUTHORIZATION_REDIS_NAME",
									Value: name,
								},
								{
									Name:  "NAMESPACE",
									Value: namespace,
								},
							},
							Command: []string{"sh", "-c"},
							Args: []string{
								`echo "Initializing Redis configuration..."
								cp /csm-auth-redis-cm/redis.conf /etc/redis/redis.conf
								echo "masterauth $REDIS_PASSWORD" >> /etc/redis/redis.conf
								echo "requirepass $REDIS_PASSWORD" >> /etc/redis/redis.conf

								MASTER_FOUND="false"
								MAX_RETRIES=5

								echo "Attempting to discover Redis master via Sentinel..."
								for retry in $(seq 0 $MAX_RETRIES)
								do
									PING_SENTINEL=$(redis-cli -h sentinel -p 5000 PING)
									echo "Pinging sentinel"

									if [ "$PING_SENTINEL" == "PONG" ]; then
										echo "Sentinel found"

										MASTER_INFO=$(redis-cli -h sentinel -p 5000 SENTINEL get-master-addr-by-name mymaster)
										MASTER_HOST=$(echo "$MASTER_INFO" | sed -n '1p')
										MASTER_PORT=$(echo "$MASTER_INFO" | sed -n '2p')
										if [ -n "$MASTER_HOST" ] && [ -n "$MASTER_PORT" ]; then
											echo "Sentinel reports master at $MASTER_HOST:$MASTER_PORT"

											# configure replicaof directive for replica pods only
											if [ "$(hostname -f)" != "$MASTER_HOST" ]; then
												echo "replicaof $MASTER_HOST $MASTER_PORT" >> /etc/redis/redis.conf
											fi

											MASTER_FOUND="true"
											break
										else
											echo "Sentinel not ready or master info missing, retrying... ($retry/$MAX_RETRIES)"
											sleep 5
										fi
									fi
								done

								# configure replicaof directive for replica pods only
								if [ "$MASTER_FOUND" != "true" ]; then
									echo "No master info from Sentinel, starting first node as master"

									MASTER_FQDN="$AUTHORIZATION_REDIS_NAME-0.$AUTHORIZATION_REDIS_NAME.$NAMESPACE.svc.cluster.local"
									if [ "$(hostname)" != "$AUTHORIZATION_REDIS_NAME-0" ]; then
										echo "replicaof $MASTER_FQDN 6379" >> /etc/redis/redis.conf
									fi
								fi
								`,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      volName,
									MountPath: "/data",
								},
								{
									Name:      "configmap",
									MountPath: "/csm-auth-redis-cm/",
								},
								{
									Name:      "config",
									MountPath: "/etc/redis/",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    name,
							Image:   image,
							Command: []string{"redis-server"},
							Args:    []string{"/etc/redis/redis.conf"},
							Ports: []corev1.ContainerPort{
								{
									Name:          name,
									ContainerPort: 6379,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      volName,
									MountPath: "/data",
								},
								{
									Name:      "configmap",
									MountPath: "/csm-auth-redis-cm/",
								},
								{
									Name:      "config",
									MountPath: "/etc/redis/",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: volName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "configmap",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "redis-csm-cm",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// getAuthorizationRediscommanderDeploymentScaffold returns a redis commander deployment for authorization v2
func getAuthorizationRediscommanderDeploymentScaffold(crName, name, namespace, image, redisSecretName, redisUsernameKey, redisPasswordKey, sentinelName, checksum string, sentinelReplicas int) appsv1.Deployment {
	runAsNonRoot := true
	readOnlyRootFilesystem := false
	allowPrivilegeEscalation := false
	var replicas int32 = 1
	return appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"csm":  crName,
						"app":  name,
						"tier": "backend",
					},
					Annotations: map[string]string{
						"checksum/secret": checksum,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "redis",
					Containers: []corev1.Container{
						{
							Name:            name,
							Image:           image,
							ImagePullPolicy: "Always",
							Env: []corev1.EnvVar{
								{
									Name:  "SENTINELS",
									Value: buildSentinelList(sentinelReplicas, sentinelName, namespace),
								},
								{
									Name:  "K8S_SIGTERM",
									Value: "1",
								},
								{
									Name: "REDIS_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: redisSecretName,
											},
											Key: redisPasswordKey,
										},
									},
								},
								{
									Name: "SENTINEL_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: redisSecretName,
											},
											Key: redisPasswordKey,
										},
									},
								},
								{
									Name: "HTTP_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: redisSecretName,
											},
											Key: redisPasswordKey,
										},
									},
								},
								{
									Name: "HTTP_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: redisSecretName,
											},
											Key: redisUsernameKey,
										},
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8081,
									Name:          name,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/favicon.png",
										Port: intstr.FromInt(8081),
									},
								},
								InitialDelaySeconds: 10,
								TimeoutSeconds:      5,
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("512Mi"),
									corev1.ResourceCPU:    resource.MustParse("500m"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             &runAsNonRoot,
								ReadOnlyRootFilesystem:   &readOnlyRootFilesystem,
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{
										"ALL",
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{},
						},
					},
					Volumes: []corev1.Volume{},
				},
			},
		},
	}
}

// getAuthorizationSentinelStatefulsetScaffold returns sentinel statefulset for authorization v2
func getAuthorizationSentinelStatefulsetScaffold(crName, sentinelName, redisName, namespace, image, redisSecretName, redisPasswordKey, checksum string, replicas int32) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      sentinelName,
			Namespace: namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: sentinelName,
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": sentinelName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"csm": crName,
						"app": sentinelName,
					},
					Annotations: map[string]string{
						"checksum/secret": checksum,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "sentinel",
					InitContainers: []corev1.Container{
						{
							Name:            "config",
							Image:           image,
							ImagePullPolicy: "Always",
							Env: []corev1.EnvVar{
								{
									Name: "REDIS_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: redisSecretName,
											},
											Key: redisPasswordKey,
										},
									},
								},
								{
									Name:  "REPLICAS",
									Value: strconv.Itoa(int(replicas)),
								},
								{
									Name:  "AUTHORIZATION_REDIS_NAME",
									Value: redisName,
								},
								{
									Name:  "AUTHORIZATION_SENTINEL_NAME",
									Value: sentinelName,
								},
								{
									Name:  "NAMESPACE",
									Value: namespace,
								},
							},
							Command: []string{"sh", "-c"},
							Args: []string{
								`MASTER_FOUND="false"
								MAX_RETRIES=5

								REPLICA=$( expr "$REPLICAS" - 1)
								SENTINELS=""
								for i in $(seq 0 $REPLICA)
								do
									SENTINEL="$AUTHORIZATION_SENTINEL_NAME-$i.$AUTHORIZATION_SENTINEL_NAME.$NAMESPACE.svc.cluster.local"
									SENTINELS="$SENTINELS $SENTINEL"
								done

								echo "Sentinel nodes: $SENTINELS"

								for retry in $(seq 0 $MAX_RETRIES)
								do
									for sentinel in $SENTINELS
									do
										echo "Querying Sentinel $SENTINEL for Redis master address..."
										MASTER_INFO=$(redis-cli -h sentinel -p 5000 SENTINEL get-master-addr-by-name mymaster)
										MASTER_HOST=$(echo "$MASTER_INFO" | sed -n '1p')
										MASTER_PORT=$(echo "$MASTER_INFO" | sed -n '2p')

										if [ -n "$MASTER_HOST" ] && [ -n "$MASTER_PORT" ]; then
											echo "Sentinel reports master at $MASTER_HOST:$MASTER_PORT"
											ROLE=$(redis-cli --no-auth-warning --raw -h "$MASTER_HOST" -p "$MASTER_PORT" -a "$REDIS_PASSWORD" ROLE | head -n 1)

											if [ "$ROLE" = "master" ]; then
												echo "Verified master role at $MASTER_HOST:$MASTER_PORT"
												MASTER=$MASTER_HOST
												MASTER_FOUND="true"
												break
											else
												echo "Role mismatch: expected master, got $ROLE"
											fi
										else
											echo "No master info from $SENTINEL"
										fi
									done

									if [ "$MASTER_FOUND" = "true" ]; then
										break
									fi

									echo "Retrying in 5 seconds... ($retry/$MAX_RETRIES)"
									sleep 5
								done

								if [ "$MASTER_FOUND" != "true" ]; then
									echo "No master found after $MAX_RETRIES retries. Defaulting to first Redis pod."
									MASTER="$AUTHORIZATION_REDIS_NAME-0.$AUTHORIZATION_REDIS_NAME.$NAMESPACE.svc.cluster.local"
								fi

								echo "Generating /etc/redis/sentinel.conf for master $MASTER"
								echo "port 5000
								sentinel resolve-hostnames yes
								sentinel announce-hostnames yes
								sentinel monitor mymaster $MASTER 6379 2
								sentinel down-after-milliseconds mymaster 5000
								sentinel failover-timeout mymaster 60000
								sentinel parallel-syncs mymaster 2
								sentinel auth-pass mymaster $REDIS_PASSWORD
								" > /etc/redis/sentinel.conf

								echo "Sentinel configuration:"
								cat /etc/redis/sentinel.conf
								`,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "redis-config",
									MountPath: "/etc/redis/",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    sentinelName,
							Image:   image,
							Command: []string{"redis-sentinel"},
							Args:    []string{"/etc/redis/sentinel.conf"},
							Ports: []corev1.ContainerPort{
								{
									Name:          sentinelName,
									ContainerPort: 5000,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "redis-config",
									MountPath: "/etc/redis/",
								},
								{
									Name:      "data",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "redis-config",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
}

// buildSentinelList builds a comma separated list of sentinel addresses
func buildSentinelList(replicas int, sentinelName, namespace string) string {
	var sentinels []string
	for i := range replicas {
		sentinel := fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local:5000", sentinelName, i, sentinelName, namespace)
		sentinels = append(sentinels, sentinel)
	}
	return strings.Join(sentinels, ", ")
}

// createRedisK8sSecret creates a k8s secret for redis
func createRedisK8sSecret(name, namespace string) corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeBasicAuth,
		StringData: map[string]string{
			"password":       "K@ravi123!",
			"commander_user": "dev",
		},
	}
}

// configVolume adds volume in a pod container for the config SecretProviderClass
func configVolume(configSecretProviderClassName string) corev1.Volume {
	volumeName := "secrets-store-inline-config"
	readOnly := true
	return corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:   "secrets-store.csi.k8s.io",
				ReadOnly: &readOnly,
				VolumeAttributes: map[string]string{
					"secretProviderClass": configSecretProviderClassName,
				},
			},
		},
	}
}

// configVolumeMount adds a volume mount in a pod container for the config SecretProviderClass
func configVolumeMount() corev1.VolumeMount {
	volumeName := "secrets-store-inline-config"
	return corev1.VolumeMount{
		Name:      volumeName,
		MountPath: "/etc/csm-authorization/config",
		ReadOnly:  true,
	}
}
