package utils

import (
	"sync/atomic"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ReconcileCSM is the interface which extends each of the respective Reconcile interfaces
// for drivers
type ReconcileCSM interface {
	reconcile.Reconciler
	GetClient() crclient.Client
	GetK8sClient() kubernetes.Interface
	GetUpdateCount() int32
	IncrUpdateCount()
}

type FakeReconcileCSM struct {
	reconcile.Reconciler
	client.Client
	K8sClient   kubernetes.Interface
	updateCount int32
}

func (r *FakeReconcileCSM) GetClient() client.Client {
	return r.Client
}

// IncrUpdateCount - Increments the update count
func (r *FakeReconcileCSM) IncrUpdateCount() {
	atomic.AddInt32(&r.updateCount, 1)
}

// GetUpdateCount - Returns the current update count
func (r *FakeReconcileCSM) GetUpdateCount() int32 {
	return r.updateCount
}

// GetK8sClient - Returns the current update count
func (r *FakeReconcileCSM) GetK8sClient() kubernetes.Interface {
	return r.K8sClient
}
