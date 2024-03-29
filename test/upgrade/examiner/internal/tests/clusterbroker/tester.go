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

package clusterbroker

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

type tester struct {
	common
	c         scClientset.ServicecatalogV1beta1Interface
	namespace string
}

func newTester(cli ClientGetter, ns string) *tester {
	return &tester{
		c:         cli.ServiceCatalogClient().ServicecatalogV1beta1(),
		namespace: ns,
		common: common{
			sc:        cli.ServiceCatalogClient().ServicecatalogV1beta1(),
			namespace: ns,
		},
	}
}

func (t *tester) execute() error {
	klog.Info("Start test resources for ServiceBroker test")
	for _, fn := range []func() error{
		t.assertClusterServiceBrokerIsReady,
		t.checkClusterServiceClass,
		t.checkClusterServicePlan,
		t.assertServiceInstanceIsReady,
		t.assertServiceBindingIsReady,
		t.removeServiceBinding,
		t.removeServiceInstance,
		t.unregisterClusterServiceBroker,
	} {
		err := fn()
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *tester) assertClusterServiceBrokerIsReady() error {
	return wait.PollUntilContextTimeout(context.Background(), waitInterval, timeoutInterval, false, func(context.Context) (done bool, err error) {
		broker, err := t.sc.ClusterServiceBrokers().Get(context.Background(), clusterServiceBrokerName, metav1.GetOptions{})
		if apiErr.IsNotFound(err) {
			klog.Infof("ClusterServiceBroker %q not exist", clusterServiceBrokerName)
			return false, nil
		}
		if err != nil {
			return false, err
		}

		condition := v1beta1.ServiceBrokerCondition{
			Type:    v1beta1.ServiceBrokerConditionReady,
			Status:  v1beta1.ConditionTrue,
			Message: successFetchedCatalogMessage,
		}
		for _, cond := range broker.Status.Conditions {
			if condition.Type == cond.Type && condition.Status == cond.Status && condition.Message == cond.Message {
				klog.Info("ClusterServiceBroker is in ready state")
				return true, nil
			}
			klog.Infof("ClusterServiceBroker is not ready, condition: Type: %q, Status: %q, Reason: %q", cond.Type, cond.Status, cond.Message)
		}

		return false, nil
	})
}

func (t *tester) removeServiceBinding() error {
	exist, err := t.serviceBindingExist()
	if err != nil {
		return fmt.Errorf("failed during fetching ServiceBinding: %w", err)
	}
	if !exist {
		return nil
	}
	if err := t.deleteServiceBinding(); err != nil {
		return fmt.Errorf("failed during removing ServiceBinding: %w", err)
	}
	if err := t.assertServiceBindingIsRemoved(); err != nil {
		return fmt.Errorf("failed during asserting ServiceBinding is removed: %w", err)
	}
	return nil
}

func (t *tester) removeServiceInstance() error {
	exist, err := t.serviceInstanceExist()
	if err != nil {
		return fmt.Errorf("failed during fetching ServiceInstance: %w", err)
	}
	if !exist {
		return nil
	}
	// remove `removeServiceInstanceFinalizer` method if BrokerTest will be fixed and
	// will handle ServiceInstance delete operation
	// for now BrokerTest failed and ServiceInstance has deprovisioning false status
	if err := t.removeServiceInstanceFinalizer(); err != nil {
		return fmt.Errorf("failed during removing ServiceInstance finalizers: %w", err)
	}
	if err := t.deleteServiceInstance(); err != nil {
		return fmt.Errorf("failed during removing ServiceInstance: %w", err)
	}
	if err := t.assertServiceInstanceIsRemoved(); err != nil {
		return fmt.Errorf("failed during asserting ServiceInstance is removed: %w", err)
	}
	return nil
}

func (t *tester) unregisterClusterServiceBroker() error {
	if err := t.deleteClusterServiceBroker(); err != nil {
		return fmt.Errorf("failed during removing ClusterServiceBroker: %w", err)
	}
	return nil
}

func (t *tester) serviceBindingExist() (bool, error) {
	_, err := t.sc.ServiceBindings(t.namespace).Get(context.Background(), serviceBindingName, metav1.GetOptions{})
	if apiErr.IsNotFound(err) {
		klog.Infof("ServiceBinding %q not exist", serviceBindingName)
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (t *tester) deleteServiceBinding() error {
	err := t.sc.ServiceBindings(t.namespace).Delete(context.Background(), serviceBindingName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (t *tester) assertServiceBindingIsRemoved() error {
	return wait.PollUntilContextTimeout(context.Background(), waitInterval, timeoutInterval, false, func(context.Context) (done bool, err error) {
		_, err = t.sc.ServiceBindings(t.namespace).Get(context.Background(), serviceBindingName, metav1.GetOptions{})
		if apiErr.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}

		return false, nil
	})
}

func (t *tester) serviceInstanceExist() (bool, error) {
	_, err := t.sc.ServiceInstances(t.namespace).Get(context.Background(), serviceInstanceName, metav1.GetOptions{})
	if apiErr.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (t *tester) removeServiceInstanceFinalizer() error {
	instance, err := t.sc.ServiceInstances(t.namespace).Get(context.Background(), serviceInstanceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	toUpdate := instance.DeepCopy()
	toUpdate.Finalizers = nil

	_, err = t.sc.ServiceInstances(toUpdate.Namespace).Update(context.Background(), toUpdate, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (t *tester) deleteServiceInstance() error {
	err := t.sc.ServiceInstances(t.namespace).Delete(context.Background(), serviceInstanceName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (t *tester) assertServiceInstanceIsRemoved() error {
	return wait.PollUntilContextTimeout(context.Background(), waitInterval, timeoutInterval, false, func(context.Context) (done bool, err error) {
		_, err = t.sc.ServiceInstances(t.namespace).Get(context.Background(), serviceInstanceName, metav1.GetOptions{})
		if apiErr.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}

		return false, nil
	})
}

func (t *tester) deleteClusterServiceBroker() error {
	err := t.sc.ClusterServiceBrokers().Delete(context.Background(), clusterServiceBrokerName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}
