/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1beta1 "github.com/drycc-addons/service-catalog/pkg/apis/servicecatalog/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeServicePlans implements ServicePlanInterface
type FakeServicePlans struct {
	Fake *FakeServicecatalogV1beta1
	ns   string
}

var serviceplansResource = v1beta1.SchemeGroupVersion.WithResource("serviceplans")

var serviceplansKind = v1beta1.SchemeGroupVersion.WithKind("ServicePlan")

// Get takes name of the servicePlan, and returns the corresponding servicePlan object, and an error if there is any.
func (c *FakeServicePlans) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.ServicePlan, err error) {
	emptyResult := &v1beta1.ServicePlan{}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithOptions(serviceplansResource, c.ns, name, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1beta1.ServicePlan), err
}

// List takes label and field selectors, and returns the list of ServicePlans that match those selectors.
func (c *FakeServicePlans) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.ServicePlanList, err error) {
	emptyResult := &v1beta1.ServicePlanList{}
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithOptions(serviceplansResource, serviceplansKind, c.ns, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.ServicePlanList{ListMeta: obj.(*v1beta1.ServicePlanList).ListMeta}
	for _, item := range obj.(*v1beta1.ServicePlanList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested servicePlans.
func (c *FakeServicePlans) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchActionWithOptions(serviceplansResource, c.ns, opts))

}

// Create takes the representation of a servicePlan and creates it.  Returns the server's representation of the servicePlan, and an error, if there is any.
func (c *FakeServicePlans) Create(ctx context.Context, servicePlan *v1beta1.ServicePlan, opts v1.CreateOptions) (result *v1beta1.ServicePlan, err error) {
	emptyResult := &v1beta1.ServicePlan{}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithOptions(serviceplansResource, c.ns, servicePlan, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1beta1.ServicePlan), err
}

// Update takes the representation of a servicePlan and updates it. Returns the server's representation of the servicePlan, and an error, if there is any.
func (c *FakeServicePlans) Update(ctx context.Context, servicePlan *v1beta1.ServicePlan, opts v1.UpdateOptions) (result *v1beta1.ServicePlan, err error) {
	emptyResult := &v1beta1.ServicePlan{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithOptions(serviceplansResource, c.ns, servicePlan, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1beta1.ServicePlan), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeServicePlans) UpdateStatus(ctx context.Context, servicePlan *v1beta1.ServicePlan, opts v1.UpdateOptions) (result *v1beta1.ServicePlan, err error) {
	emptyResult := &v1beta1.ServicePlan{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(serviceplansResource, "status", c.ns, servicePlan, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1beta1.ServicePlan), err
}

// Delete takes name of the servicePlan and deletes it. Returns an error if one occurs.
func (c *FakeServicePlans) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(serviceplansResource, c.ns, name, opts), &v1beta1.ServicePlan{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeServicePlans) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithOptions(serviceplansResource, c.ns, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1beta1.ServicePlanList{})
	return err
}

// Patch applies the patch and returns the patched servicePlan.
func (c *FakeServicePlans) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.ServicePlan, err error) {
	emptyResult := &v1beta1.ServicePlan{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(serviceplansResource, c.ns, name, pt, data, opts, subresources...), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1beta1.ServicePlan), err
}
