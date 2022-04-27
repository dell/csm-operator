package clientgoclient

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	applyconfigurationscorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FakeConfigMaps implements ConfigMapInterface
type FakeConfigMaps struct {
	FakeClient client.Client
	Namespace  string
}

// Get takes name of the configMap, and returns the corresponding configMap object, and an error if there is any.
func (c *FakeConfigMaps) Get(ctx context.Context, name string, options v1.GetOptions) (result *corev1.ConfigMap, err error) {

	result = new(corev1.ConfigMap)

	k := types.NamespacedName{
		Name:      name,
		Namespace: c.Namespace,
	}

	err = c.FakeClient.Get(ctx, k, result)
	return
}

// List takes label and field selectors, and returns the list of ConfigMaps that match those selectors.
func (c *FakeConfigMaps) List(ctx context.Context, opts v1.ListOptions) (result *corev1.ConfigMapList, err error) {
	panic("implement me")
}

// Watch returns a watch.Interface that watches the requested configMaps.
func (c *FakeConfigMaps) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

// Create takes the representation of a configMap and creates it.  Returns the server's representation of the configMap, and an error, if there is any.
func (c *FakeConfigMaps) Create(ctx context.Context, configMap *corev1.ConfigMap, opts v1.CreateOptions) (result *corev1.ConfigMap, err error) {
	err = c.FakeClient.Create(ctx, configMap)
	fmt.Printf("create fake configmap %s", configMap.Name)
	return configMap, err
}

// Update takes the representation of a configMap and updates it. Returns the server's representation of the configMap, and an error, if there is any.
func (c *FakeConfigMaps) Update(ctx context.Context, configMap *corev1.ConfigMap, opts v1.UpdateOptions) (result *corev1.ConfigMap, err error) {
	panic("implement me")
}

// Delete takes name of the configMap and deletes it. Returns an error if one occurs.
func (c *FakeConfigMaps) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	panic("implement me")
}

// DeleteCollection deletes a collection of objects.
func (c *FakeConfigMaps) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	panic("implement me")
}

// Patch applies the patch and returns the patched configMap.
func (c *FakeConfigMaps) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *corev1.ConfigMap, err error) {
	panic("implement me")
}

// Apply takes the given apply declarative configuration, applies it and returns the applied configMap.
func (c *FakeConfigMaps) Apply(ctx context.Context, configMap *applyconfigurationscorev1.ConfigMapApplyConfiguration, opts v1.ApplyOptions) (result *corev1.ConfigMap, err error) {
	result = new(corev1.ConfigMap)

	data, err := json.Marshal(configMap)
	if err != nil {
		return result, err
	}

	err = json.Unmarshal(data, result)

	_, err = c.Get(ctx, *configMap.Name, v1.GetOptions{})
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
