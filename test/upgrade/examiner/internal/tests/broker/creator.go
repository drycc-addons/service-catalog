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

	"github.com/kubernetes-sigs/service-catalog/pkg/apis/servicecatalog/v1beta1"
	scClientset "github.com/kubernetes-sigs/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

type creator struct {
	common
	sc        scClientset.ServicecatalogV1beta1Interface
	namespace string
}

func newCreator(cli ClientGetter, ns string) *creator {
	return &creator{
		sc:        cli.ServiceCatalogClient().ServicecatalogV1beta1(),
		namespace: ns,
		common: common{
			sc:        cli.ServiceCatalogClient().ServicecatalogV1beta1(),
			namespace: ns,
		},
	}
}

func (c *creator) execute() error {
	klog.Info("Start prepare resources for ServiceBroker test")
	for _, fn := range []func() error{
		c.registerServiceBroker,
		c.checkServiceClass,
		c.checkServicePlan,
		c.createServiceInstance,
		c.createServiceBinding,
	} {
		err := fn()
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *creator) registerServiceBroker() error {
	klog.Infof("Create ServiceBroker %q", serviceBrokerName)

	_, err := c.sc.ServiceBrokers(c.namespace).Create(context.Background(), &v1beta1.ServiceBroker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceBrokerName,
			Namespace: c.namespace,
		},
		Spec: v1beta1.ServiceBrokerSpec{
			CommonServiceBrokerSpec: v1beta1.CommonServiceBrokerSpec{
				URL: "http://test-broker-test-broker.test-broker.svc.cluster.local",
			},
		},
	}, metav1.CreateOptions{})

	if err != nil {
		return fmt.Errorf("failed during creating ServiceBroker: %w", err)
	}

	return nil
}

func (c *creator) createServiceInstance() error {
	klog.Info("Create ServiceInstance")
	if err := c.createDefaultServiceInstance(); err != nil {
		return fmt.Errorf("failed during creating ServiceInstance: %w", err)
	}

	klog.Info("Check ServiceInstance is ready")
	if err := c.assertServiceInstanceIsReady(); err != nil {
		return fmt.Errorf("failed during checking ServiceInstance conditions: %w", err)
	}

	return nil
}

func (c *creator) createDefaultServiceInstance() error {
	_, err := c.sc.ServiceInstances(c.namespace).Create(context.Background(), &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceInstanceName,
			Namespace: c.namespace,
		},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ServiceClassExternalName: "test-service-multiple-plans",
				ServicePlanExternalName:  "default",
			},
			Parameters: &runtime.RawExtension{
				Raw: []byte(`{ "param-1":"value-1", "param-2":"value-2" }`),
			},
		},
	}, metav1.CreateOptions{})

	return err
}

func (c *creator) createServiceBinding() error {
	klog.Info("Create ServiceBinding")
	if err := c.createDefaultServiceBinding(); err != nil {
		return fmt.Errorf("failed during creating ServiceBinding: %w", err)
	}

	klog.Info("Check ServiceBinding is ready")
	if err := c.assertServiceBindingIsReady(); err != nil {
		return fmt.Errorf("failed during checking ServiceBinding conditions: %w", err)
	}

	return nil
}

func (c *creator) createDefaultServiceBinding() error {
	_, err := c.sc.ServiceBindings(c.namespace).Create(context.Background(), &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceBindingName,
			Namespace: c.namespace,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{
				Name: serviceInstanceName,
			},
		},
	}, metav1.CreateOptions{})

	return err
}
