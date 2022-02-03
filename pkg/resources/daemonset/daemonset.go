package daemonset

import (
	"context"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"k8s.io/client-go/kubernetes"
)

// SyncDaemonset - Syncs a daemonset object
func SyncDaemonset(ctx context.Context, daemonset *appsv1.DaemonSetApplyConfiguration, k8sClient kubernetes.Interface, reqLogger logr.Logger, csmName string) error {
	reqLogger.Info("Sync DaemonSet:", "name", *daemonset.ObjectMetaApplyConfiguration.Name)

	// Get a config to talk to the apiserver
	daemonsets := k8sClient.AppsV1().DaemonSets(*daemonset.ObjectMetaApplyConfiguration.Namespace)

	found, err := daemonsets.Get(ctx, *daemonset.ObjectMetaApplyConfiguration.Name, metav1.GetOptions{})
	if err != nil {
		reqLogger.Info("get SyncDaemonset error", "Error", err.Error())
	}

	opts := metav1.ApplyOptions{FieldManager: "application/apply-patch"}
	if found == nil || found.Name == "" {
		reqLogger.Info("No existing DaemonSet", "Name:", daemonset.Name)
	} else {
		reqLogger.Info("found daemonset", "image", found.Spec.Template.Spec.Containers[0].Image)
	}

	daemonset.Spec.Template.Labels["csm"] = csmName
	set, err := daemonsets.Apply(ctx, daemonset, opts)
	if err != nil {
		reqLogger.Info("Apply DaemonSet error", "set", err.Error())
		return err
	}
	reqLogger.Info("daemonset apply done", "name", set.Name)
	return nil
}
