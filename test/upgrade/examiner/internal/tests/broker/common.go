/*
Copyright 2019 The Kubernetes Authors.

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

package broker

import (
	"context"
	"fmt"

	"github.com/drycc-addons/service-catalog/pkg/apis/servicecatalog/v1beta1"
	scClientset "github.com/drycc-addons/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1beta1"
	apiErr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

type common struct {
	sc        scClientset.ServicecatalogV1beta1Interface
	namespace string
}

func (c *common) checkServiceClass() error {
	klog.Info("Check ServiceClasses")
	if err := c.assertProperAmountOfServiceClasses(); err != nil {
		return fmt.Errorf("failed during list ServiceClasses: %w", err)
	}

	return nil
}

func (c *common) assertProperAmountOfServiceClasses() error {
	return wait.PollUntilContextTimeout(context.Background(), waitInterval, timeoutInterval, false, func(context.Context) (done bool, err error) {
		list, err := c.sc.ServiceClasses(c.namespace).List(context.Background(), metav1.ListOptions{})
		if apiErr.IsNotFound(err) {
			klog.Info("ServiceClasses not exist")
			return false, nil
		}
		if err != nil {
			return false, err
		}

		amount := len(list.Items)
		if amount == amountOfServiceClasses {
			klog.Infof("All expected elements (%d) exists: %d items", amountOfServiceClasses, amount)
			return true, nil
		}

		klog.Errorf("There should be %d ServiceClassess, have %d", amountOfServiceClasses, amount)
		return false, nil
	})
}

func (c *common) checkServicePlan() error {
	klog.Info("Check ServicePlans")
	if err := c.assertProperAmountOfServicePlans(); err != nil {
		return fmt.Errorf("failed during list ServiceClasses: %w", err)
	}

	return nil
}

func (c *common) assertProperAmountOfServicePlans() error {
	return wait.PollUntilContextTimeout(context.Background(), waitInterval, timeoutInterval, false, func(ctx context.Context) (done bool, err error) {
		list, err := c.sc.ServicePlans(c.namespace).List(ctx, metav1.ListOptions{})
		if apiErr.IsNotFound(err) {
			klog.Info("ServicePlans not exist")
			return false, nil
		}
		if err != nil {
			return false, err
		}

		amount := len(list.Items)
		if amount == amountOfServicePlans {
			klog.Infof("All expected elements (%d) exists: %d items", amountOfServicePlans, amount)
			return true, nil
		}

		klog.Errorf("There should be %d ServicePlans, have %d", amountOfServicePlans, amount)
		return false, nil
	})
}

func (c *common) assertServiceInstanceIsReady() error {
	return wait.PollUntilContextTimeout(context.Background(), waitInterval, timeoutInterval, false, func(context.Context) (done bool, err error) {
		instance, err := c.sc.ServiceInstances(c.namespace).Get(context.Background(), serviceInstanceName, metav1.GetOptions{})
		if apiErr.IsNotFound(err) {
			klog.Infof("ServiceInstance %q not exist", serviceInstanceName)
			return false, nil
		}
		if err != nil {
			return false, err
		}

		condition := v1beta1.ServiceInstanceCondition{
			Type:    v1beta1.ServiceInstanceConditionReady,
			Status:  v1beta1.ConditionTrue,
			Message: successProvisionMessage,
		}
		for _, cond := range instance.Status.Conditions {
			if condition.Type == cond.Type && condition.Status == cond.Status && condition.Message == cond.Message {
				klog.Info("ServiceInstance is in ready state")
				return true, nil
			}
			klog.Infof("ServiceInstance is not ready, condition: Type: %q, Status: %q, Reason: %q", cond.Type, cond.Status, cond.Message)
		}

		return false, nil
	})
}

func (c *common) assertServiceBindingIsReady() error {
	return wait.PollUntilContextTimeout(context.Background(), waitInterval, timeoutInterval, false, func(context.Context) (done bool, err error) {
		binding, err := c.sc.ServiceBindings(c.namespace).Get(context.Background(), serviceBindingName, metav1.GetOptions{})
		if apiErr.IsNotFound(err) {
			klog.Infof("ServiceBinding %q not exist", serviceBindingName)
			return false, nil
		}
		if err != nil {
			return false, err
		}

		condition := v1beta1.ServiceBindingCondition{
			Type:    v1beta1.ServiceBindingConditionReady,
			Status:  v1beta1.ConditionTrue,
			Message: successInjectedBindResultMessage,
		}
		for _, cond := range binding.Status.Conditions {
			if condition.Type == cond.Type && condition.Status == cond.Status && condition.Message == cond.Message {
				klog.Info("ServiceBinding is in ready state")
				return true, nil
			}
			klog.Infof("ServiceBinding is not ready, condition: Type: %q, Status: %q, Reason: %q", cond.Type, cond.Status, cond.Message)
		}

		return false, nil
	})
}
