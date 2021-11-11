package service

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncService - Creates/Updates a service
func SyncService(ctx context.Context, service *corev1.Service, client client.Client, reqLogger logr.Logger) error {
	found := &corev1.Service{}
	err := client.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Service", "Name", service.Name)
		err = client.Create(ctx, service)
		if err != nil {
			return err
		}
	} else if err != nil {
		reqLogger.Info("Unknown error.", "Error", err.Error())
		return err
	} else {
		reqLogger.Info("Updating Service", "Name:", service.Name)
		err = client.Update(ctx, service)
		if err != nil {
			return err
		}
	}

	return nil
}
