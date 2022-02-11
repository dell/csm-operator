package configmap

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/dell/csm-operator/pkg/logger"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncConfigMap - Creates/Updates a config map
func SyncConfigMap(ctx context.Context, configMap *corev1.ConfigMap, client client.Client) error {
	log := logger.GetLogger(ctx)

	found := &corev1.ConfigMap{}
	err := client.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Infow("Creating a new ConfigMap", "Name", configMap.Name)
		err = client.Create(ctx, configMap)
		if err != nil {
			return err
		}
	} else if err != nil {
		log.Errorw("Unknown error.", "Error", err.Error())
		return err
	} else {
		log.Infow("Updating ConfigMap", "Name:", configMap.Name)
		err = client.Update(ctx, configMap)
		if err != nil {
			return err
		}
	}

	return nil
}
