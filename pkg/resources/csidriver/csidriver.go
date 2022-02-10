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
func SyncCSIDriver(ctx context.Context, csi *storagev1.CSIDriver, client client.Client, csmName string, trcID string) error {
	//log := logger.GetLogger(ctx)
	name := csmName + "-" + trcID
	_, log := logger.GetNewContextWithLogger(name)

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
				log.Error(err, "Failed to update CSIDriver object")
			} else {
				log.Infow("Successfully updated CSIDriver object", "Name:", csi.Name)
			}
		}
	}
	return nil
}
