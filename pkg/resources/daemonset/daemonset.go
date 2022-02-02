package daemonset

import (
	"context"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// SyncDaemonset - Syncs a daemonset object
func SyncDaemonset(ctx context.Context, daemonset *applv1.DaemonSetApplyConfiguration, client client.Client, reqLogger logr.Logger, csmName string) error {
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
