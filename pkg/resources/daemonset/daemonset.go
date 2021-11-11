package daemonset

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncDaemonset - Syncs a daemonset object
func SyncDaemonset(ctx context.Context, daemonset *appsv1.DaemonSet, client client.Client, reqLogger logr.Logger) error {
	//fmt.Println("Creating DaemonSet:", daemonset.Name, daemonset.Namespace)
	found := &appsv1.DaemonSet{}
	err := client.Get(ctx, types.NamespacedName{Name: daemonset.Name, Namespace: daemonset.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new DaemonSet", "Namespace", daemonset.Namespace, "Name", daemonset.Name)
		err = client.Create(ctx, daemonset)
		if err != nil {
			return err
		}
	} else if err != nil {
		reqLogger.Info("Unknown error.", "Error", err.Error())
		return err
	} else {
		reqLogger.Info("Updating DaemonSet", "Name:", daemonset.Name)
		err = client.Update(ctx, daemonset)
		if err != nil {
			return err
		}
	}
	return nil
}

func isValidDNSPolicy(str string) bool {
	allowedDNSPolicies := []string{string(corev1.DNSClusterFirst), string(corev1.DNSClusterFirstWithHostNet),
		string(corev1.DNSNone), string(corev1.DNSDefault)}

	for _, v := range allowedDNSPolicies {
		if v == str {
			return true
		}
	}
	return false
}
