package csidriver

import (
	"context"

	storagev1 "k8s.io/api/storage/v1"

	"github.com/dell/csm-operator/pkg/logger"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncCSIDriver - Syncs a CSI Driver object
func SyncCSIDriver(ctx context.Context, csi *storagev1.CSIDriver, client client.Client) error {
	log := logger.GetLogger(ctx)
	found := &storagev1.CSIDriver{}
	err := client.Get(ctx, types.NamespacedName{Name: csi.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Infow("Creating a new CSIDriver", "Name:", csi.Name)
		err = client.Create(ctx, csi)
		if err != nil {
			return err
		}
	} else if err != nil {
		log.Errorw("Unknown error.", "Error", err.Error())
		return err
	} else {
		log.Infow("CSIDriver Object exist", "Name:", csi.Name)
	}
	csi.Labels["driverVersion"] = "v2.2.0"
	return nil
}

