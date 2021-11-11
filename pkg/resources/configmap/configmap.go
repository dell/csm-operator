package configmap

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncConfigMap - Creates/Updates a config map
func SyncConfigMap(ctx context.Context, configMap *corev1.ConfigMap, client client.Client, reqLogger logr.Logger) error {
	found := &corev1.ConfigMap{}
	err := client.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new ConfigMap", "Name", configMap.Name)
		err = client.Create(ctx, configMap)
		if err != nil {
			return err
		}
	} else if err != nil {
		reqLogger.Info("Unknown error.", "Error", err.Error())
		return err
	} else {
		reqLogger.Info("Updating ConfigMap", "Name:", configMap.Name)
		err = client.Update(ctx, configMap)
		if err != nil {
			return err
		}
	}

	return nil
}
