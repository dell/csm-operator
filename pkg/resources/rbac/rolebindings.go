package rbac

import (
	"context"

	"github.com/dell/csm-operator/pkg/logger"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncClusterRoleBindings - Syncs the ClusterRoleBindings
func SyncClusterRoleBindings(ctx context.Context, rb rbacv1.ClusterRoleBinding, client client.Client) error {
	log := logger.GetLogger(ctx)
	found := &rbacv1.ClusterRoleBinding{}
	err := client.Get(ctx, types.NamespacedName{Name: rb.Name, Namespace: rb.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new ClusterRoleBinding", "Namespace", rb.Namespace, "Name", rb.Name)
		err = client.Create(ctx, &rb)
		if err != nil {
			return err
		}
	} else if err != nil {
		log.Info("Unknown error.", "Error", err.Error())
		return err
	} else {
		log.Info("Updating ClusterRoleBinding", "Name:", rb.Name)
		err = client.Update(ctx, &rb)
		if err != nil {
			return err
		}
	}
	return nil
}
