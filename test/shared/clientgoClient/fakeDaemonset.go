package clientgoClient

import (
	"context"
	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	applyconfigurationsappsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FakeDaemonSets implements DaemonSetInterface
type FakeDaemonSets struct {
	OperatorClient client.Client
	Namespace      string
}

// Apply takes the given apply declarative configuration, applies it and returns the applied daemonSet.
func (c *FakeDaemonSets) Apply(ctx context.Context, daemonSet *applyconfigurationsappsv1.DaemonSetApplyConfiguration, opts v1.ApplyOptions) (result *appsv1.DaemonSet, err error) {
	result = new(appsv1.DaemonSet)

	data, err := json.Marshal(daemonSet)
	if err != nil {
		return result, err
	}

	json.Unmarshal(data, result)

	_, err = c.Get(ctx, *daemonSet.Name, v1.GetOptions{})
	if errors.IsNotFound(err) {
		// if not found, we create it
		return c.Create(ctx, result, v1.CreateOptions{})
	} else if err != nil {
		return 
	}

	// otherwise we update it
	err = c.OperatorClient.Update(ctx, result)

	return result, err
}

// Get takes name of the daemonSet, and returns the corresponding daemonSet object, and an error if there is any.
func (c *FakeDaemonSets) Get(ctx context.Context, name string, options v1.GetOptions) (result *appsv1.DaemonSet, err error) {
	result = new(appsv1.DaemonSet)

	k := types.NamespacedName{
		Name:      name,
		Namespace: c.Namespace,
	}

	err = c.OperatorClient.Get(ctx, k, result)
	return
}

// Create takes the representation of a daemonSet and creates it.  Returns the server's representation of the daemonSet, and an error, if there is any.
func (c *FakeDaemonSets) Create(ctx context.Context, daemonSet *appsv1.DaemonSet, opts v1.CreateOptions) (result *appsv1.DaemonSet, err error) {
	err = c.OperatorClient.Create(ctx, daemonSet)
	return daemonSet, err
}

// List takes label and field selectors, and returns the list of DaemonSets that match those selectors.
func (c *FakeDaemonSets) List(ctx context.Context, opts v1.ListOptions) (result *appsv1.DaemonSetList, err error) {
	panic("implement me")
}

// Watch returns a watch.Interface that watches the requested daemonSets.
func (c *FakeDaemonSets) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

// Update takes the representation of a daemonSet and updates it. Returns the server's representation of the daemonSet, and an error, if there is any.
func (c *FakeDaemonSets) Update(ctx context.Context, daemonSet *appsv1.DaemonSet, opts v1.UpdateOptions) (result *appsv1.DaemonSet, err error) {
	panic("implement me")
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeDaemonSets) UpdateStatus(ctx context.Context, daemonSet *appsv1.DaemonSet, opts v1.UpdateOptions) (*appsv1.DaemonSet, error) {
	panic("implement me")
}

// Delete takes name of the daemonSet and deletes it. Returns an error if one occurs.
func (c *FakeDaemonSets) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	panic("implement me")
}

// DeleteCollection deletes a collection of objects.
func (c *FakeDaemonSets) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	panic("implement me")
}

// Patch applies the patch and returns the patched daemonSet.
func (c *FakeDaemonSets) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *appsv1.DaemonSet, err error) {
	panic("implement me")
}

// ApplyStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
func (c *FakeDaemonSets) ApplyStatus(ctx context.Context, daemonSet *applyconfigurationsappsv1.DaemonSetApplyConfiguration, opts v1.ApplyOptions) (result *appsv1.DaemonSet, err error) {
	panic("implement me")
}
