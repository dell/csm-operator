package daemonset

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	//appsv1 "k8s.io/api/apps/v1"
	//"k8s.io/apimachinery/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apps "k8s.io/client-go/applyconfigurations/apps/v1"
	//acorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	//applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// SyncDaemonset - Syncs a daemonset object
func SyncDaemonset(ctx context.Context, daemonset *apps.DaemonSetApplyConfiguration, client client.Client, reqLogger logr.Logger) error {
	reqLogger.Info("Sync DaemonSet:", "name", *daemonset.ObjectMetaApplyConfiguration.Name)

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	clientset, err := v1.NewForConfig(cfg)
	daemonsets := clientset.DaemonSets(*daemonset.ObjectMetaApplyConfiguration.Namespace)

	found, err := daemonsets.Get(ctx, *daemonset.ObjectMetaApplyConfiguration.Name, metav1.GetOptions{})
	if err != nil {
		reqLogger.Info("get SyncDaemonset error", "Error", err.Error())
	}
	opts := metav1.ApplyOptions{FieldManager: "application/apply-patch"}
	if found.Name == "" {
		reqLogger.Info("No existing DaemonSet", "Name:", found.Name)

	} else {
		fmt.Printf("debug found daemonset env %#v \n", found.Spec.Template.Spec.Containers[0].Image)
	}
	fmt.Printf("debug daemonset image %s \n", *daemonset.Spec.Template.Spec.Containers[0].Image)
	// if i change a value here , it works , gets applied
	for _, e := range daemonset.Spec.Template.Spec.Containers[0].Env {
		/*
			if *e.Name == "X_CSI_MAX_VOLUMES_PER_NODE" {
				*e.Value = "4"
			}
		*/
		if e.Value != nil {
			fmt.Printf("debug daemonset env name %s=%s \n", *e.Name, *e.Value)
		}
	}
	set, err := daemonsets.Apply(ctx, daemonset, opts)
	if err != nil {
		reqLogger.Info("Apply DaemonSet error", "set", err.Error())
		return err
	}
	fmt.Printf("debug daemonset status %#v \n", set.Status)
	return nil
}
