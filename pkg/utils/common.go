package utils

import (
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ReconcileCSM is the interface which extends each of the respective Reconcile interfaces
// for drivers
//go:generate mockery --name=ReconcileCSM
type ReconcileCSM interface {
	reconcile.Reconciler
	GetClient() crclient.Client
	GetUpdateCount() int32
	IncrUpdateCount()
}
