package csidriver

import (
	"context"

	storagev1 "k8s.io/api/storage/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncCSIDriver - Syncs a CSI Driver object
func SyncCSIDriver(ctx context.Context, csi *storagev1.CSIDriver, client client.Client, reqLogger logr.Logger) error {
	found := &storagev1.CSIDriver{}
	err := client.Get(ctx, types.NamespacedName{Name: csi.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new CSIDriver", "Name:", csi.Name)
		err = client.Create(ctx, csi)
		if err != nil {
			return err
		}
	} else if err != nil {
		reqLogger.Info("Unknown error.", "Error", err.Error())
		return err
	} else {
		isUpdateRequired := false
		ownerRefs := found.GetOwnerReferences()
		for _, ownerRef := range ownerRefs {
			if ownerRef.APIVersion != "rbac.authorization.k8s.io/v1" {
				// Lets overwrite everything
				isUpdateRequired = true
				break
			}
		}
		if isUpdateRequired {
			found.OwnerReferences = csi.OwnerReferences
			err = client.Update(ctx, found)
			if err != nil {
				reqLogger.Error(err, "Failed to update CSIDriver object")
			} else {
				reqLogger.Info("Successfully updated CSIDriver object", "Name:", csi.Name)
			}
		}
	}
	return nil
}
