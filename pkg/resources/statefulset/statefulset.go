package statefulset

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SleepTime - minimum time to sleep before checking the state of controller pod
var SleepTime = 10 * time.Second

// DeleteStatefulset -- Deletes a StatefulSet
func DeleteStatefulset(ctx context.Context, statefulset *appsv1.StatefulSet, client client.Client, reqLogger logr.Logger) error {
	err := client.Delete(ctx, statefulset)
	if err != nil && errors.IsNotFound(err) {
		return err
	}
	return nil
}

// SyncStatefulset - Syncs a StatefulSet
func SyncStatefulset(ctx context.Context, statefulset *appsv1.StatefulSet, client client.Client, reqLogger logr.Logger) error {
	found := &appsv1.StatefulSet{}
	err := client.Get(ctx, types.NamespacedName{Name: statefulset.Name, Namespace: statefulset.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Statefulset", "Namespace", statefulset.Namespace, "Name", statefulset.Name)
		err = client.Create(ctx, statefulset)
		if err != nil {
			return err
		}

		return nil
	} else if err != nil {
		reqLogger.Info("Unknown error.", "Error", err.Error())
		return err
	} else {
		reqLogger.Info("Updating StatefulSet", "Name:", statefulset.Name)
		err = client.Update(ctx, statefulset)
		if err != nil {
			return err
		}
		if statefulset.Status.ReadyReplicas != statefulset.Status.Replicas {
			// Check if the pod spec is same as pod spec from stateful spec
			reqLogger.Info("Waiting 10 seconds before checking the status of controller pods")
			time.Sleep(SleepTime)
		}
		err := client.Get(ctx, types.NamespacedName{Name: statefulset.Name, Namespace: statefulset.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to find the statefulset after upgrade. Internal error!")
			return err
		}
		podTemplateSpec := found.Spec.Template.Spec
		for i := found.Status.Replicas - 1; i >= 0; i-- {
			controllerPod := &corev1.Pod{}
			controllerPodName := fmt.Sprintf("%s-%d", statefulset.Name, i)
			err = client.Get(ctx, types.NamespacedName{Name: controllerPodName, Namespace: statefulset.Namespace}, controllerPod)
			if err == nil {
				podSpec := controllerPod.Spec
				if !comparePodSpec(podTemplateSpec, podSpec, reqLogger) {
					reqLogger.Info("Deleting the controller pod", controllerPodName)
					err = client.Delete(ctx, controllerPod)
					if err != nil {
						reqLogger.Error(err, "Failed to delete the pod. Continuing")
					}
				}
			} else {
				reqLogger.Error(err, "Failed to get the controller pod. Continuing")
			}
		}
	}
	return nil
}

func comparePodSpec(spec1, spec2 corev1.PodSpec, reqLogger logr.Logger) bool {
	for _, container1 := range spec1.Containers {
		for _, container2 := range spec2.Containers {
			if container1.Name == container2.Name {
				if !reflect.DeepEqual(container1.Env, container2.Env) {
					reqLogger.Info("Environments don't match for", container1.Name)
					return false
				}
				reqLogger.Info(fmt.Sprintf("Environment variables match for %s", container1.Name))
				if container1.Image != container2.Image {
					reqLogger.Info(fmt.Sprintf("Image (%s, %s) don't match for container %s",
						string(container1.Image), string(container2.Image), container1.Name))
					return false
				}
				if container1.ImagePullPolicy != container2.ImagePullPolicy {
					reqLogger.Info(fmt.Sprintf("ImagePullPolicy (%s, %s) don't match for container %s",
						string(container1.ImagePullPolicy), string(container2.ImagePullPolicy), container1.Name))
					return false
				}
			}
			break
		}
	}
	return true
}
