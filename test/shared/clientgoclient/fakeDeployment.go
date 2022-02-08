package clientgoclient

import (
	"context"
	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	applyconfigurationsappsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	applyconfigurationsautoscalingv1 "k8s.io/client-go/applyconfigurations/autoscaling/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FakeDeployments implements DeploymentInterface
type FakeDeployments struct {
	FakeClient client.Client
	Namespace  string
}

// Apply takes the given apply declarative configuration, applies it and returns the applied deployment.
func (c *FakeDeployments) Apply(ctx context.Context, deployment *applyconfigurationsappsv1.DeploymentApplyConfiguration, opts v1.ApplyOptions) (result *appsv1.Deployment, err error) {
	result = new(appsv1.Deployment)

	data, err := json.Marshal(deployment)
	if err != nil {
		return result, err
	}

	json.Unmarshal(data, result)

	_, err = c.Get(ctx, *deployment.Name, v1.GetOptions{})
	if errors.IsNotFound(err) {
		// if not found, we create it
		return c.Create(ctx, result, v1.CreateOptions{})
	} else if err != nil {
		return
	}

	// otherwise we update it
	err = c.FakeClient.Update(ctx, result)

	return result, err
}

// Get takes name of the deployment, and returns the corresponding deployment object, and an error if there is any.
func (c *FakeDeployments) Get(ctx context.Context, name string, options v1.GetOptions) (result *appsv1.Deployment, err error) {
	result = new(appsv1.Deployment)

	k := types.NamespacedName{
		Name:      name,
		Namespace: c.Namespace,
	}

	err = c.FakeClient.Get(ctx, k, result)
	return
}

// Create takes the representation of a deployment and creates it. Returns the server's representation of the deployment, and an error, if there is any.
func (c *FakeDeployments) Create(ctx context.Context, deployment *appsv1.Deployment, opts v1.CreateOptions) (result *appsv1.Deployment, err error) {
	err = c.FakeClient.Create(ctx, deployment)
	return deployment, err
}

// List takes label and field selectors, and returns the list of Deployments that match those selectors.
func (c *FakeDeployments) List(ctx context.Context, opts v1.ListOptions) (result *appsv1.DeploymentList, err error) {
	panic("implement me")
}

// Watch returns a watch.Interface that watches the requested deployments.
func (c *FakeDeployments) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

// Update takes the representation of a deployment and updates it. Returns the server's representation of the deployment, and an error, if there is any.
func (c *FakeDeployments) Update(ctx context.Context, deployment *appsv1.Deployment, opts v1.UpdateOptions) (result *appsv1.Deployment, err error) {
	panic("implement me")
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeDeployments) UpdateStatus(ctx context.Context, deployment *appsv1.Deployment, opts v1.UpdateOptions) (*appsv1.Deployment, error) {
	panic("implement me")
}

// Delete takes name of the deployment and deletes it. Returns an error if one occurs.
func (c *FakeDeployments) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	panic("implement me")
}

// DeleteCollection deletes a collection of objects.
func (c *FakeDeployments) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	panic("implement me")
}

// Patch applies the patch and returns the patched deployment.
func (c *FakeDeployments) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *appsv1.Deployment, err error) {
	panic("implement me")
}

// ApplyStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
func (c *FakeDeployments) ApplyStatus(ctx context.Context, deployment *applyconfigurationsappsv1.DeploymentApplyConfiguration, opts v1.ApplyOptions) (result *appsv1.Deployment, err error) {
	panic("implement me")
}

// GetScale takes name of the deployment, and returns the corresponding scale object, and an error if there is any.
func (c *FakeDeployments) GetScale(ctx context.Context, deploymentName string, options v1.GetOptions) (result *autoscalingv1.Scale, err error) {
	panic("implement me")
}

// UpdateScale takes the representation of a scale and updates it. Returns the server's representation of the scale, and an error, if there is any.
func (c *FakeDeployments) UpdateScale(ctx context.Context, deploymentName string, scale *autoscalingv1.Scale, opts v1.UpdateOptions) (result *autoscalingv1.Scale, err error) {
	panic("implement me")
}

// ApplyScale takes top resource name and the apply declarative configuration for scale,
// applies it and returns the applied scale, and an error, if there is any.
func (c *FakeDeployments) ApplyScale(ctx context.Context, deploymentName string, scale *applyconfigurationsautoscalingv1.ScaleApplyConfiguration, opts v1.ApplyOptions) (result *autoscalingv1.Scale, err error) {
	panic("implement me")
}
