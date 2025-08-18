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

	csmv1 "github.com/dell/csm-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// getProxyServerScaffold returns proxy-server deployment for authorization v2
func getProxyServerScaffold(name, sentinelName, namespace, proxyImage, opaImage, opaKubeMgmtImage, jwtSigningSecretName, redisSecretName, redisPasswordKey string, replicas int32, sentinelReplicas int) appsv1.Deployment {
	var volumeMounts = []corev1.VolumeMount{
		{
			Name:      "csm-config-params",
			MountPath: "/etc/karavi-authorization/csm-config-params",
		},
	}
	var volumes = []corev1.Volume{
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
	}

	if jwtSigningSecretName == "" {
		var configVolumeMnt = corev1.VolumeMount{
			Name:      "config-volume",
			MountPath: "/etc/karavi-authorization/config",
		}
		var configVolume = corev1.Volume{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "karavi-config-secret",
				},
			},
		}
		volumeMounts = append(volumeMounts, configVolumeMnt)
		volumes = append(volumes, configVolume)
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
								fmt.Sprintf("--redis-sentinel=%s", buildSentinelList(sentinelReplicas, sentinelName, namespace)),
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
func getStorageServiceScaffold(name string, namespace string, image string, replicas int32, jwtSigningSecretName string) appsv1.Deployment {
	var volumeMounts = []corev1.VolumeMount{
		{
			Name:      "csm-config-params",
			MountPath: "/etc/karavi-authorization/csm-config-params",
		},
	}
	var volumes = []corev1.Volume{
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
	}

	if jwtSigningSecretName == "" {
		var configVolumeMnt = corev1.VolumeMount{
			Name:      "config-volume",
			MountPath: "/etc/karavi-authorization/config",
		}
		var configVolume = corev1.Volume{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "karavi-config-secret",
				},
			},
		}
		volumeMounts = append(volumeMounts, configVolumeMnt)
		volumes = append(volumes, configVolume)
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
func getTenantServiceScaffold(name, namespace, seninelName, image, jwtSigningSecretName, redisSecretName, redisPasswordKey string, replicas int32, sentinelReplicas int) appsv1.Deployment {
	var volumeMounts = []corev1.VolumeMount{
		{
			Name:      "csm-config-params",
			MountPath: "/etc/karavi-authorization/csm-config-params",
		},
	}
	var volumes = []corev1.Volume{
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
	}

	if jwtSigningSecretName == "" {
		var configVolumeMnt = corev1.VolumeMount{
			Name:      "config-volume",
			MountPath: "/etc/karavi-authorization/config",
		}
		var configVolume = corev1.Volume{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "karavi-config-secret",
				},
			},
		}
		volumeMounts = append(volumeMounts, configVolumeMnt)
		volumes = append(volumes, configVolume)
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
								fmt.Sprintf("--redis-sentinel=%s", buildSentinelList(sentinelReplicas, seninelName, namespace)),
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
							},
							Command: []string{"sh", "-c"},
							Args: []string{
								`cp /csm-auth-redis-cm/redis.conf /etc/redis/redis.conf
								echo "masterauth $REDIS_PASSWORD" >> /etc/redis/redis.conf
								echo "requirepass $REDIS_PASSWORD" >> /etc/redis/redis.conf
								echo "Finding master..."
								MASTER_FDQN=$(hostname  -f | sed -e 's/redis-csm-[0-9]\./redis-csm-0./')
								echo "Master at " $MASTER_FDQN
								if [ "$(redis-cli -h sentinel -p 5000 ping)" != "PONG" ]; then
									echo "No sentinel found."
									if [ "$(hostname)" = "redis-csm-0" ]; then
									echo "This is redis master, not updating config..."
									else
									echo "This is redis slave, updating redis.conf..."
									echo "replicaof $MASTER_FDQN 6379" >> /etc/redis/redis.conf
									fi
								else
									echo "Sentinel found, finding master"
									MASTER="$(redis-cli -h sentinel -p 5000 sentinel get-master-addr-by-name mymaster | grep -E '(^redis-csm-\d{1,})|([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})')"
									echo "replicaof $MASTER_FDQN 6379" >> /etc/redis/redis.conf
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
										Name: "redis-cm",
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
									Value: fmt.Sprintf("%s", buildSentinelList(sentinelReplicas, sentinelName, namespace)),
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
							},
							Command: []string{"sh", "-c"},
							Args: []string{
								`replicas=$( expr "$REPLICAS" - 1)
								nodes=""
								for i in $(seq 0 "$replicas")
								do
									node=$( echo "$AUTHORIZATION_REDIS_NAME"-$i."$AUTHORIZATION_REDIS_NAME" )
									nodes=$( echo "$nodes*$node" )
								done
								loop=$(echo $nodes | sed -e "s/"*"/\n/g")
								echo "$loop"
								foundMaster=false
								while [ "$foundMaster" = "false" ]
								do
									for i in $loop
									do
										echo "Finding master at $i"
										ROLE=$(redis-cli --no-auth-warning --raw -h $i -a $REDIS_PASSWORD info replication | awk '{print $1}' | grep role | cut -d ":" -f2)
										if [ "$ROLE" = "master" ]; then
											MASTER=$i.authorization.svc.cluster.local
											echo "Master found at $MASTER..."
											foundMaster=true
											break
										else
										MASTER=$(redis-cli --no-auth-warning --raw -h $i -a $REDIS_PASSWORD info replication | awk '{print $1}' | grep master_host: | cut -d ":" -f2)
										if [ "$MASTER" = "" ]; then
											echo "Master not found..."
											echo "Waiting 5 seconds for redis pods to come up..."
											sleep 5
											MASTER=
										else
											echo "Master found at $MASTER..."
											foundMaster=true
											break
										fi
										fi
									done
									if [ "$foundMaster" = "true" ]; then
									break
									else
									echo "Master not found, wait for 30s before attempting again"
									sleep 30
									fi
								done
								echo "sentinel monitor mymaster $MASTER 6379 2" >> /tmp/master
								echo "port 5000
								sentinel resolve-hostnames yes
								sentinel announce-hostnames yes
								$(cat /tmp/master)
								sentinel down-after-milliseconds mymaster 5000
								sentinel failover-timeout mymaster 60000
								sentinel parallel-syncs mymaster 2
								sentinel auth-pass mymaster $REDIS_PASSWORD
								" > /etc/redis/sentinel.conf
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
	for i := 0; i < replicas; i++ {
		sentinel := fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local:5000", sentinelName, i, sentinelName, namespace)
		sentinels = append(sentinels, sentinel)
	}
	return strings.Join(sentinels, ",")
}

// createRedisK8sSecret creates a k8s secret for redis
func createRedisK8sSecret(cr csmv1.ContainerStorageModule, usernameKey, passwordKey string) corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultRedisSecretName,
			Namespace: cr.Namespace,
		},
		Type: corev1.SecretTypeBasicAuth,
		StringData: map[string]string{
			passwordKey: "K@ravi123!",
			usernameKey: "dev",
		},
	}
}

// redisVolume adds volume in a pod container for the redis SecretProviderClass
func redisVolume(redisSecretName string) corev1.Volume {
	volumeName := "secrets-store-inline-redis"
	readOnly := true
	return corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:   "secrets-store.csi.k8s.io",
				ReadOnly: &readOnly,
				VolumeAttributes: map[string]string{
					"secretProviderClass": redisSecretName,
				},
			},
		},
	}
}

// redisVolumeMount adds a volume mount in a pod container for the redis SecretProviderClass
func redisVolumeMount() corev1.VolumeMount {
	volumeName := "secrets-store-inline-redis"
	return corev1.VolumeMount{
		Name:      volumeName,
		MountPath: "/etc/csm-authorization/redis",
		ReadOnly:  true,
	}
}

// jwtVolume adds volume in a pod container for the jwt signing secret SecretProviderClass
func jwtVolume(jwtSigningSecretName string) corev1.Volume {
	volumeName := "secrets-store-inline-jwt"
	readOnly := true
	return corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:   "secrets-store.csi.k8s.io",
				ReadOnly: &readOnly,
				VolumeAttributes: map[string]string{
					"secretProviderClass": jwtSigningSecretName,
				},
			},
		},
	}
}

// jwtVolumeMount adds a volume mount in a pod container for the jwt signing secret SecretProviderClass
func jwtVolumeMount() corev1.VolumeMount {
	volumeName := "secrets-store-inline-jwt"
	return corev1.VolumeMount{
		Name:      volumeName,
		MountPath: "/etc/csm-authorization/config",
		ReadOnly:  true,
	}
}
