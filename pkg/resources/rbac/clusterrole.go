package rbac

import (
	"context"

	"github.com/dell/csm-operator/pkg/logger"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncClusterRole - Syncs a ClusterRole
func SyncClusterRole(ctx context.Context, clusterRole rbacv1.ClusterRole, client client.Client) error {
	log := logger.GetLogger(ctx)
	found := &rbacv1.ClusterRole{}
	err := client.Get(ctx, types.NamespacedName{Name: clusterRole.Name, Namespace: clusterRole.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new ClusterRole", "Name", clusterRole.Name)
		err = client.Create(ctx, &clusterRole)
		if err != nil {
			return err
		}
		// we need to return found object
		err := client.Get(ctx, types.NamespacedName{Name: clusterRole.Name, Namespace: clusterRole.Namespace}, found)
		if err != nil {
			return err
		}
	} else if err != nil {
		log.Info("Unknown error.", "Error", err.Error())
		return err
	} else {
		log.Info("Updating ClusterRole", "Name:", clusterRole.Name)
		err = client.Update(ctx, &clusterRole)
		if err != nil {
			return err
		}
	}

	return nil
}
