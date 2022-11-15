//  Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package crclient

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"reflect"

	"github.com/dell/csm-operator/tests/shared"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Client implements a controller runtime client
// Objects mocks k8s resources
// ErrorInjector is used to force errors from controller for test
type Client struct {
	Objects       map[shared.StorageKey]runtime.Object
	ErrorInjector shared.ErrorInjector
}

// NewFakeClient creates a new client
func NewFakeClient(objectMap map[shared.StorageKey]runtime.Object, errorInjector shared.ErrorInjector) *Client {
	return &Client{
		Objects:       objectMap,
		ErrorInjector: errorInjector,
	}
}

// NewFakeClientNoInjector creates a new client without an error injector
func NewFakeClientNoInjector(objectMap map[shared.StorageKey]runtime.Object) *Client {
	return &Client{Objects: objectMap}
}

// Get implements client.Client.
func (f Client) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if f.ErrorInjector != nil {
		if err := f.ErrorInjector.ShouldFail("Get", obj); err != nil {
			return err
		}
	}

	gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
	if err != nil {
		return err
	}
	k := shared.StorageKey{
		Name:      key.Name,
		Namespace: key.Namespace,
		Kind:      gvk.Kind,
	}
	o, found := f.Objects[k]
	if !found {
		gvr := schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}
		return errors.NewNotFound(gvr, key.Name)
	}

	j, err := json.Marshal(o)
	if err != nil {
		return err
	}
	decoder := scheme.Codecs.UniversalDecoder()
	_, _, err = decoder.Decode(j, nil, obj)
	return err
}

// List implements client.Client.
func (f Client) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if f.ErrorInjector != nil {
		if err := f.ErrorInjector.ShouldFail("List", list); err != nil {
			return err
		}
	}
	switch list.(type) {
	case *corev1.PodList:
		return f.listPodList(list.(*corev1.PodList))
	case *appsv1.DeploymentList:
		return f.listDeploymentList(ctx, &appsv1.DeploymentList{})
	default:
		return fmt.Errorf("fake client unknown type: %s", reflect.TypeOf(list))
	}
}

func (f Client) listPodList(list *corev1.PodList) error {
	for k, v := range f.Objects {
		if k.Kind == "Pod" {
			list.Items = append(list.Items, *v.(*corev1.Pod))
		}
	}
	return nil
}

func (f Client) listDeploymentList(ctx context.Context, list *appsv1.DeploymentList) error {
	for k, v := range f.Objects {
		if k.Kind == "Deployment" {
			list.Items = append(list.Items, *v.(*appsv1.Deployment))
		}
	}
	return nil
}

// Create implements client.Client.
func (f Client) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if f.ErrorInjector != nil {
		if err := f.ErrorInjector.ShouldFail("Create", obj); err != nil {
			return err
		}
	}
	k, err := shared.GetKey(obj)
	if err != nil {
		return err
	}
	_, found := f.Objects[k]
	if found {
		gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
		if err != nil {
			return err
		}
		gvr := schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}
		return errors.NewAlreadyExists(gvr, k.Name)
	}
	f.Objects[k] = obj
	return nil
}

// Delete implements client.Client.
func (f Client) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if f.ErrorInjector != nil {
		if err := f.ErrorInjector.ShouldFail("Delete", obj); err != nil {
			return err
		}
	}
	if len(opts) > 0 {
		return fmt.Errorf("delete options are not supported")
	}
	if f.ErrorInjector != nil {
		if err := f.ErrorInjector.ShouldFail("Delete", obj); err != nil {
			return err
		}
	}

	k, err := shared.GetKey(obj)
	if err != nil {
		return err
	}
	_, found := f.Objects[k]
	if !found {
		gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
		if err != nil {
			return err
		}
		gvr := schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}
		return errors.NewNotFound(gvr, k.Name)
	}

	// if deletiontimestamp is not zero, we want to go into deletion logic
	if !obj.GetDeletionTimestamp().IsZero() {
		return nil
	}

	delete(f.Objects, k)
	return nil
}

// Clear cleans objects
func (f Client) Clear() {
	for sk := range f.Objects {
		delete(f.Objects, sk)
	}
}

// SetDeletionTimeStamp so that reconcile can go into deletion part of code
func (f Client) SetDeletionTimeStamp(ctx context.Context, obj client.Object) error {
	k, err := shared.GetKey(obj)
	if err != nil {
		return err
	}

	if len(obj.GetFinalizers()) > 0 {
		obj.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
		f.Objects[k] = obj
		return nil
	}

	return fmt.Errorf("failed to set timestamp")
}

// Update implements client.StatusWriter.
func (f Client) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if f.ErrorInjector != nil {
		if err := f.ErrorInjector.ShouldFail("Update", obj); err != nil {
			return err
		}
	}
	k, err := shared.GetKey(obj)
	if err != nil {
		return err
	}
	_, found := f.Objects[k]
	if !found {
		gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
		if err != nil {
			return err
		}
		gvr := schema.GroupResource{
			Group:    gvk.Group,
			Resource: gvk.Kind,
		}
		return errors.NewNotFound(gvr, k.Name)
	}
	f.Objects[k] = obj
	return nil
}

// Patch implements client.Client.
func (f Client) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	panic("implement me")
}

// DeleteAllOf implements client.Client.
func (f Client) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	panic("implement me")
}

// Status implements client.StatusClient.
func (f Client) Status() client.StatusWriter {
	return f
}

// Scheme returns the scheme this client is using.
func (f Client) Scheme() *runtime.Scheme {
	return scheme.Scheme
}

// RESTMapper returns the scheme this client is using.
func (f Client) RESTMapper() meta.RESTMapper {
	panic("implement me")
}
