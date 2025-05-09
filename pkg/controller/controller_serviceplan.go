/*
Copyright 2017 The Kubernetes Authors.

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

package controller

import (
	"context"

	"github.com/drycc-addons/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/drycc-addons/service-catalog/pkg/util"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"github.com/drycc-addons/service-catalog/pkg/pretty"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// Service plan handlers and control-loop

func (c *controller) servicePlanAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("ServicePlan: Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.servicePlanQueue.Add(key)
}

func (c *controller) servicePlanUpdate(oldObj, newObj interface{}) {
	c.servicePlanAdd(newObj)
}

func (c *controller) servicePlanDelete(obj interface{}) {
	servicePlan, ok := obj.(*v1beta1.ServicePlan)
	if servicePlan == nil || !ok {
		return
	}

	klog.V(4).Infof("ServicePlan: Received delete event for %v; no further processing will occur", servicePlan.Name)
}

// reconcileServicePlanKey reconciles a ServicePlan due to resync
// or an event on the ServicePlan.  Note that this is NOT the main
// reconciliation loop for ServicePlans. ServicePlans are primarily
//
//	reconciled in a separate flow when a ServiceBroker is reconciled.
func (c *controller) reconcileServicePlanKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	pcb := pretty.NewContextBuilder(pretty.ServicePlan, namespace, name, "")
	plan, err := c.servicePlanLister.ServicePlans(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.Info(pcb.Message("not doing work because plan has been deleted"))
		return nil
	}
	if err != nil {
		klog.Info(pcb.Message("unable to retrieve object from store: %v"))
		return err
	}

	return c.reconcileServicePlan(plan)
}

func (c *controller) reconcileServicePlan(servicePlan *v1beta1.ServicePlan) error {
	pcb := pretty.NewContextBuilder(pretty.ServicePlan, servicePlan.Namespace, servicePlan.Name, "")
	klog.Infof("ServicePlan %q (ExternalName: %q): processing", servicePlan.Name, servicePlan.Spec.ExternalName)

	if !servicePlan.Status.RemovedFromBrokerCatalog {
		return nil
	}

	klog.Info(pcb.Message("removed from broker catalog; determining whether there are instances remaining"))

	serviceInstances, err := c.findServiceInstancesOnServicePlan(servicePlan)
	if err != nil {
		return err
	}
	klog.Info(pcb.Messagef("Found %d ServiceInstances", len(serviceInstances.Items)))

	if len(serviceInstances.Items) != 0 {
		return nil
	}

	klog.Info(pcb.Message("removed from broker catalog and has zero instances remaining; deleting"))
	return c.serviceCatalogClient.ServicePlans(servicePlan.Namespace).Delete(context.Background(), servicePlan.Name, metav1.DeleteOptions{})
}

func (c *controller) findServiceInstancesOnServicePlan(servicePlan *v1beta1.ServicePlan) (*v1beta1.ServiceInstanceList, error) {
	labelSelector := labels.SelectorFromSet(labels.Set{
		v1beta1.GroupName + "/" + v1beta1.FilterSpecServicePlanRefName: util.GenerateSHA(servicePlan.Name),
	}).String()

	listOpts := metav1.ListOptions{
		LabelSelector: labelSelector,
	}

	return c.serviceCatalogClient.ServiceInstances(metav1.NamespaceAll).List(context.Background(), listOpts)
}
