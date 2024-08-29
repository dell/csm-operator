//  Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//       http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

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

// FakeStatefulsets implements StatefulSetInterface
type FakeStatefulsets struct {
	FakeClient client.Client
	Namespace  string
}

// Apply takes the given apply declarative configuration, applies it and returns the applied statefulset.
func (c *FakeStatefulsets) Apply(ctx context.Context, statefulset *applyconfigurationsappsv1.StatefulSetApplyConfiguration, _ v1.ApplyOptions) (result *appsv1.StatefulSet, err error) {
	result = new(appsv1.StatefulSet)

	data, err := json.Marshal(statefulset)
	if err != nil {
		return result, err
	}

	_ = json.Unmarshal(data, result)

	_, err = c.Get(ctx, *statefulset.Name, v1.GetOptions{})
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

// Get takes name of the statefulset, and returns the corresponding statefulset object, and an error if there is any.
func (c *FakeStatefulsets) Get(ctx context.Context, name string, _ v1.GetOptions) (result *appsv1.StatefulSet, err error) {
	result = new(appsv1.StatefulSet)

	k := types.NamespacedName{
		Name:      name,
		Namespace: c.Namespace,
	}

	err = c.FakeClient.Get(ctx, k, result)
	return
}

// Create takes the representation of a statefulset and creates it. Returns the server's representation of the statefulset, and an error, if there is any.
func (c *FakeStatefulsets) Create(ctx context.Context, statefulset *appsv1.StatefulSet, _ v1.CreateOptions) (result *appsv1.StatefulSet, err error) {
	err = c.FakeClient.Create(ctx, statefulset)
	return statefulset, err
}

// List takes label and field selectors, and returns the list of StatefulSets that match those selectors.
func (c *FakeStatefulsets) List(_ context.Context, _ v1.ListOptions) (result *appsv1.StatefulSetList, err error) {
	panic("implement me")
}

// Watch returns a watch.Interface that watches the requested statefulsets.
func (c *FakeStatefulsets) Watch(_ context.Context, _ v1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

// Update takes the representation of a statefulset and updates it. Returns the server's representation of the statefulset, and an error, if there is any.
func (c *FakeStatefulsets) Update(_ context.Context, _ *appsv1.StatefulSet, _ v1.UpdateOptions) (result *appsv1.StatefulSet, err error) {
	panic("implement me")
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeStatefulsets) UpdateStatus(_ context.Context, _ *appsv1.StatefulSet, _ v1.UpdateOptions) (*appsv1.StatefulSet, error) {
	panic("implement me")
}

// Delete takes name of the statefulset and deletes it. Returns an error if one occurs.
func (c *FakeStatefulsets) Delete(_ context.Context, _ string, _ v1.DeleteOptions) error {
	panic("implement me")
}

// DeleteCollection deletes a collection of objects.
func (c *FakeStatefulsets) DeleteCollection(_ context.Context, _ v1.DeleteOptions, _ v1.ListOptions) error {
	panic("implement me")
}

// Patch applies the patch and returns the patched statefulset.
func (c *FakeStatefulsets) Patch(_ context.Context, _ string, _ types.PatchType, _ []byte, _ v1.PatchOptions, _ ...string) (result *appsv1.StatefulSet, err error) {
	panic("implement me")
}

// ApplyStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
func (c *FakeStatefulsets) ApplyStatus(_ context.Context, _ *applyconfigurationsappsv1.StatefulSetApplyConfiguration, _ v1.ApplyOptions) (result *appsv1.StatefulSet, err error) {
	panic("implement me")
}

// GetScale takes name of the statefulset, and returns the corresponding scale object, and an error if there is any.
func (c *FakeStatefulsets) GetScale(_ context.Context, _ string, _ v1.GetOptions) (result *autoscalingv1.Scale, err error) {
	panic("implement me")
}

// UpdateScale takes the representation of a scale and updates it. Returns the server's representation of the scale, and an error, if there is any.
func (c *FakeStatefulsets) UpdateScale(_ context.Context, _ string, _ *autoscalingv1.Scale, _ v1.UpdateOptions) (result *autoscalingv1.Scale, err error) {
	panic("implement me")
}

// ApplyScale takes top resource name and the apply declarative configuration for scale,
// applies it and returns the applied scale, and an error, if there is any.
func (c *FakeStatefulsets) ApplyScale(_ context.Context, _ string, _ *applyconfigurationsautoscalingv1.ScaleApplyConfiguration, _ v1.ApplyOptions) (result *autoscalingv1.Scale, err error) {
	panic("implement me")
}

// AutoscalingV2 takes top resource name and the apply declarative configuration for scale,
// applies it and returns the applied scale, and an error, if there is any.
func (c *FakeStatefulsets) AutoscalingV2(_ context.Context, _ string, _ *applyconfigurationsautoscalingv1.ScaleApplyConfiguration, _ v1.ApplyOptions) (result *autoscalingv1.Scale, err error) {
	panic("implement me")
}
