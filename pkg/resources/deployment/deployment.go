package deployment

import (
	"context"
	//"fmt"

	"github.com/dell/csm-operator/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"k8s.io/client-go/kubernetes"

	//"reflect"
	"time"
)

// SleepTime - minimum time to sleep before checking the state of controller pod
var SleepTime = 10 * time.Second

// SyncDeployment - Syncs a Deployment for controller
func SyncDeployment(ctx context.Context, deployment *appsv1.DeploymentApplyConfiguration, k8sClient kubernetes.Interface, csmName string) error {
	log := logger.GetLogger(ctx)

	log.Infow("Sync Deployment:", "name", *deployment.ObjectMetaApplyConfiguration.Name)

	deployments := k8sClient.AppsV1().Deployments(*deployment.ObjectMetaApplyConfiguration.Namespace)

	found, err := deployments.Get(ctx, *deployment.ObjectMetaApplyConfiguration.Name, metav1.GetOptions{})
	if err != nil {
		log.Errorw("get SyncDeployment error", "Error", err.Error())
	}
	opts := metav1.ApplyOptions{FieldManager: "application/apply-patch"}
	if found.Name == "" {
		log.Infow("No existing Deployment", "Name:", found.Name)

	} else {
		log.Infow("found deployment", "image", found.Spec.Template.Spec.Containers[0].Image)
	}

	deployment.Spec.Template.Labels["csm"] = csmName
	set, err := deployments.Apply(ctx, deployment, opts)
	if err != nil {
		log.Errorw("Apply Deployment error", "set", err.Error())
		return err
	}
	log.Infow("deployment apply done", "name", set.Name)
	return nil
}
