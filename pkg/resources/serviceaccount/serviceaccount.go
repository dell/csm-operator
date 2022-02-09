package serviceaccount

import (
	"context"
	"github.com/dell/csm-operator/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncServiceAccount - Syncs a ServiceAccount
func SyncServiceAccount(ctx context.Context, sa *corev1.ServiceAccount, client client.Client) error {
	log := logger.GetLogger(ctx)
	found := &corev1.ServiceAccount{}
	err := client.Get(ctx, types.NamespacedName{Name: sa.Name, Namespace: sa.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new ServiceAccount", "Namespace", sa.Namespace, "Name", sa.Name)
		err = client.Create(ctx, sa)
		if err != nil {
			return err
		}

		return nil
	} else if err != nil {
		log.Info("Unknown error.", "Error", err.Error())
		return err
	} else {
		log.Info("Updating ServiceAccount", "Name:", sa.Name)
		err = client.Update(ctx, sa)
		if err != nil {
			return err
		}
	}
	return nil
}
