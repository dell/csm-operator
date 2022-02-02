package deployment

import (
	"context"
	//"fmt"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apps "k8s.io/client-go/applyconfigurations/apps/v1"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	//"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"time"
)

// SleepTime - minimum time to sleep before checking the state of controller pod
var SleepTime = 10 * time.Second

// SyncDeployment - Syncs a Deployment for controller
func SyncDeployment(ctx context.Context, deployment *apps.DeploymentApplyConfiguration, client client.Client, reqLogger logr.Logger, csmName string) error {
	reqLogger.Info("Sync Deployment:", "name", *deployment.ObjectMetaApplyConfiguration.Name)

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	clientset, err := v1.NewForConfig(cfg)
	deployments := clientset.Deployments(*deployment.ObjectMetaApplyConfiguration.Namespace)

	found, err := deployments.Get(ctx, *deployment.ObjectMetaApplyConfiguration.Name, metav1.GetOptions{})
	if err != nil {
		reqLogger.Info("get SyncDeployment error", "Error", err.Error())
	}
	opts := metav1.ApplyOptions{FieldManager: "application/apply-patch"}
	if found.Name == "" {
		reqLogger.Info("No existing Deployment", "Name:", found.Name)

	} else {
		reqLogger.Info("found deployment", "image", found.Spec.Template.Spec.Containers[0].Image)
	}

	deployment.Spec.Template.Labels["csm"] = csmName
	set, err := deployments.Apply(ctx, deployment, opts)
	if err != nil {
		reqLogger.Info("Apply Deployment error", "set", err.Error())
		return err
	}
	reqLogger.Info("deployment apply done", "name", set.Name)
	return nil
}
