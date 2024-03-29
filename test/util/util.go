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

package util

import (
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/drycc-addons/service-catalog/pkg/apis/servicecatalog/v1beta1"
	v1beta1servicecatalog "github.com/drycc-addons/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1beta1"
	scfeatures "github.com/drycc-addons/service-catalog/pkg/features"
	servicecatalog "github.com/drycc-addons/service-catalog/pkg/svcat/service-catalog"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

// WaitForBrokerCondition waits for the status of the named broker to contain
// a condition whose type and status matches the supplied one. Checks for a
// ClusterServiceBroker by default, a ServiceBroker if a namespace is provided
func WaitForBrokerCondition(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, name string, condition v1beta1.ServiceBrokerCondition, namespace ...string) error {
	// GetCatalog default timeout time is 60 seconds, so the wait here must be at least that (previously set to 30 seconds)
	var err error
	var broker servicecatalog.Broker
	return wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, 3*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			if len(namespace) == 0 {
				klog.V(5).Infof("Waiting for ClusterServiceBroker %v condition %#v", name, condition)
				broker, err = client.ClusterServiceBrokers().Get(ctx, name, metav1.GetOptions{})
			} else {
				klog.V(5).Infof("Waiting for ServiceBroker %v in namespace %v to have condition %#v", name, namespace[0], condition)
				broker, err = client.ServiceBrokers(namespace[0]).Get(ctx, name, metav1.GetOptions{})
			}
			if nil != err {
				return false, fmt.Errorf("error getting Broker %v: %v", name, err)
			}

			if len(broker.GetStatus().Conditions) == 0 {
				return false, nil
			}

			klog.V(5).Infof("Conditions = %#v", broker.GetStatus().Conditions)

			for _, cond := range broker.GetStatus().Conditions {
				if condition.Type == cond.Type && condition.Status == cond.Status {
					if condition.Reason == "" || condition.Reason == cond.Reason {
						return true, nil
					}
				}
			}

			return false, nil
		},
	)
}

// WaitForBrokerToNotExist waits for the Broker with the given name to no
// longer exist. Checks for ClusterServiceBrokers by default, ServiceBrokers
// if a namespace is provided
func WaitForBrokerToNotExist(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, name string, namespace ...string) error {
	var err error
	return wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, wait.ForeverTestTimeout, true,
		func(ctx context.Context) (bool, error) {
			if len(namespace) == 0 {
				klog.V(5).Infof("Waiting for ClusterServiceBroker %v to not exist", name)
				_, err = client.ClusterServiceBrokers().Get(ctx, name, metav1.GetOptions{})
			} else {
				klog.V(5).Infof("Waiting for ServiceBroker %v in namespace %v to not exist", name, namespace[0])
				_, err = client.ServiceBrokers(namespace[0]).Get(ctx, name, metav1.GetOptions{})
			}
			if nil == err {
				return false, nil
			}

			if errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForServiceClassToExist waits for the ServiceClass with the given name
// to exist. Checks for a ClusterServiceClass by default, a ServiceClass if
// a namespace is provided
func WaitForServiceClassToExist(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, name string, namespace ...string) error {
	return wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, wait.ForeverTestTimeout, true,
		func(ctx context.Context) (bool, error) {
			var err error
			if len(namespace) == 0 {
				klog.V(5).Infof("Waiting for ClusterServiceClass %v to exist", name)
				_, err = client.ClusterServiceClasses().Get(ctx, name, metav1.GetOptions{})
			} else {
				klog.V(5).Infof("Waiting for ServiceClass %v in namespace %v to exist", name, namespace[0])
				_, err = client.ServiceClasses(namespace[0]).Get(ctx, name, metav1.GetOptions{})
			}
			if nil == err {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForServicePlanToExist waits for the ServicePlan
// with the given name to exist. Checks for ClusterServicePlans
// by default, ServicePlans if a namespace is provided
func WaitForServicePlanToExist(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, name string, namespace ...string) error {
	return wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, wait.ForeverTestTimeout, true,
		func(ctx context.Context) (bool, error) {
			var err error
			if len(namespace) == 0 {
				klog.V(5).Infof("Waiting for ClusterServicePlan %v to exist", name)
				_, err = client.ClusterServicePlans().Get(ctx, name, metav1.GetOptions{})
			} else {
				klog.V(5).Infof("Waiting for ServicePlan %v in namespace %v to exist", name, namespace[0])
				_, err = client.ServicePlans(namespace[0]).Get(ctx, name, metav1.GetOptions{})
			}
			if nil == err {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForServicePlanToNotExist waits for the plan with the given name
// to not exist. Looks for ClusterServicePlans by default, ServicePlans if a
// namespace is provided
func WaitForServicePlanToNotExist(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, name string, namespace ...string) error {
	return wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, wait.ForeverTestTimeout, true,
		func(ctx context.Context) (bool, error) {
			var err error
			if len(namespace) == 0 {
				klog.V(5).Infof("Waiting for ClusterServicePlan %q to not exist", name)
				_, err = client.ClusterServicePlans().Get(ctx, name, metav1.GetOptions{})
			} else {
				klog.V(5).Infof("Waiting for ServicePlan %q in namespace %v to not exist", name, namespace[0])
				_, err = client.ServicePlans(namespace[0]).Get(ctx, name, metav1.GetOptions{})
			}
			if nil == err {
				return false, nil
			}

			if errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForServiceClassToNotExist waits for the class with the given
// name to no longer exist. Looks for ClusterServiceClasses by default,
// ServiceClasses if a namespace is provided
func WaitForServiceClassToNotExist(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, name string, namespace ...string) error {
	return wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, wait.ForeverTestTimeout, true,
		func(ctx context.Context) (bool, error) {
			var err error
			if len(namespace) == 0 {
				klog.V(5).Infof("Waiting for ClusterServiceClass %v to not exist", name)
				_, err = client.ClusterServiceClasses().Get(ctx, name, metav1.GetOptions{})
			} else {
				klog.V(5).Infof("Waiting for ServiceClass %v in namespace %v to not exist", name, namespace[0])
				_, err = client.ServiceClasses(namespace[0]).Get(ctx, name, metav1.GetOptions{})
			}
			if nil == err {
				return false, nil
			}

			if errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForInstanceCondition waits for the status of the named instance to
// contain a condition whose type and status matches the supplied one.
func WaitForInstanceCondition(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, namespace, name string, condition v1beta1.ServiceInstanceCondition) error {
	return wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, wait.ForeverTestTimeout, true,
		func(ctx context.Context) (bool, error) {
			klog.V(5).Infof("Waiting for instance %v/%v condition %#v", namespace, name, condition)
			instance, err := client.ServiceInstances(namespace).Get(ctx, name, metav1.GetOptions{})
			if nil != err {
				return false, fmt.Errorf("error getting Instance %v/%v: %v", namespace, name, err)
			}

			if len(instance.Status.Conditions) == 0 {
				return false, nil
			}

			klog.V(5).Infof("Conditions = %#v", instance.Status.Conditions)

			for _, cond := range instance.Status.Conditions {
				if condition.Type == cond.Type && condition.Status == cond.Status {
					if condition.Reason == "" || condition.Reason == cond.Reason {
						return true, nil
					}
				}
			}

			return false, nil
		},
	)
}

// WaitForInstanceToNotExist waits for the Instance with the given name to no
// longer exist.
func WaitForInstanceToNotExist(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, namespace, name string) error {
	return wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, wait.ForeverTestTimeout, true,
		func(ctx context.Context) (bool, error) {
			klog.V(5).Infof("Waiting for instance %v/%v to not exist", namespace, name)

			_, err := client.ServiceInstances(namespace).Get(ctx, name, metav1.GetOptions{})
			if nil == err {
				return false, nil
			}

			if errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForInstanceProcessedGeneration waits for the status of the named instance to
// have the specified reconciled generation.
func WaitForInstanceProcessedGeneration(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, namespace, name string, processedGeneration int64) error {
	return wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, wait.ForeverTestTimeout, true,
		func(ctx context.Context) (bool, error) {
			klog.V(5).Infof("Waiting for instance %v/%v to have processed generation of %v", namespace, name, processedGeneration)
			instance, err := client.ServiceInstances(namespace).Get(ctx, name, metav1.GetOptions{})
			if nil != err {
				return false, fmt.Errorf("error getting Instance %v/%v: %v", namespace, name, err)
			}

			if instance.Status.ObservedGeneration >= processedGeneration &&
				(isServiceInstanceReady(instance) || isServiceInstanceFailed(instance)) &&
				!instance.Status.OrphanMitigationInProgress {
				return true, nil
			}

			return false, nil
		},
	)
}

// isServiceInstanceConditionTrue returns whether the given instance has a given condition
// with status true.
func isServiceInstanceConditionTrue(instance *v1beta1.ServiceInstance, conditionType v1beta1.ServiceInstanceConditionType) bool {
	for _, cond := range instance.Status.Conditions {
		if cond.Type == conditionType {
			return cond.Status == v1beta1.ConditionTrue
		}
	}

	return false
}

// isServiceInstanceReady returns whether the given instance has a ready condition
// with status true.
func isServiceInstanceReady(instance *v1beta1.ServiceInstance) bool {
	return isServiceInstanceConditionTrue(instance, v1beta1.ServiceInstanceConditionReady)
}

// isServiceInstanceFailed returns whether the instance has a failed condition with
// status true.
func isServiceInstanceFailed(instance *v1beta1.ServiceInstance) bool {
	return isServiceInstanceConditionTrue(instance, v1beta1.ServiceInstanceConditionFailed)
}

// WaitForBindingCondition waits for the status of the named binding to contain
// a condition whose type and status matches the supplied one and then returns
// back the last binding condition of the same type requested during polling if found.
func WaitForBindingCondition(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, namespace, name string, condition v1beta1.ServiceBindingCondition) (*v1beta1.ServiceBindingCondition, error) {
	var lastSeenCondition *v1beta1.ServiceBindingCondition
	return lastSeenCondition, wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, wait.ForeverTestTimeout, true,
		func(ctx context.Context) (bool, error) {
			klog.V(5).Infof("Waiting for binding %v/%v condition %#v", namespace, name, condition)

			binding, err := client.ServiceBindings(namespace).Get(ctx, name, metav1.GetOptions{})
			if nil != err {
				return false, fmt.Errorf("error getting Binding %v/%v: %v", namespace, name, err)
			}

			if len(binding.Status.Conditions) == 0 {
				return false, nil
			}

			klog.V(5).Infof("Conditions = %#v", binding.Status.Conditions)

			for _, cond := range binding.Status.Conditions {
				if condition.Type == cond.Type {
					lastSeenCondition = &cond
				}
				if condition.Type == cond.Type && condition.Status == cond.Status {
					if condition.Reason == "" || condition.Reason == cond.Reason {
						return true, nil
					}
				}
			}

			return false, nil
		},
	)
}

// WaitForBindingToNotExist waits for the Binding with the given name to no
// longer exist.
func WaitForBindingToNotExist(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, namespace, name string) error {
	return wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, wait.ForeverTestTimeout, true,
		func(ctx context.Context) (bool, error) {
			klog.V(5).Infof("Waiting for binding %v/%v to not exist", namespace, name)

			_, err := client.ServiceBindings(namespace).Get(ctx, name, metav1.GetOptions{})
			if nil == err {
				return false, nil
			}

			if errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		},
	)
}

// WaitForBindingReconciledGeneration waits for the status of the named binding to
// have the specified reconciled generation.
func WaitForBindingReconciledGeneration(client v1beta1servicecatalog.ServicecatalogV1beta1Interface, namespace, name string, reconciledGeneration int64) error {
	return wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, wait.ForeverTestTimeout, true,
		func(ctx context.Context) (bool, error) {
			klog.V(5).Infof("Waiting for binding %v/%v to have reconciled generation of %v", namespace, name, reconciledGeneration)
			binding, err := client.ServiceBindings(namespace).Get(ctx, name, metav1.GetOptions{})
			if nil != err {
				return false, fmt.Errorf("error getting ServiceBinding %v/%v: %v", namespace, name, err)
			}

			if binding.Status.ReconciledGeneration == reconciledGeneration {
				return true, nil
			}

			return false, nil
		},
	)
}

// AssertServiceInstanceCondition asserts that the instance's status contains
// the given condition type, status, and reason.
func AssertServiceInstanceCondition(t *testing.T, instance *v1beta1.ServiceInstance, conditionType v1beta1.ServiceInstanceConditionType, status v1beta1.ConditionStatus, reason ...string) {
	foundCondition := false
	for _, condition := range instance.Status.Conditions {
		if condition.Type == conditionType {
			foundCondition = true
			if condition.Status != status {
				t.Fatalf("%v condition had unexpected status; expected %v, got %v", conditionType, status, condition.Status)
			}
			if len(reason) == 1 && condition.Reason != reason[0] {
				t.Fatalf("unexpected reason; expected %v, got %v", reason[0], condition.Reason)
			}
		}
	}

	if !foundCondition {
		t.Fatalf("%v condition not found", conditionType)
	}
}

// AssertServiceBindingCondition asserts that the binding's status contains
// the given condition type, status, and reason.
func AssertServiceBindingCondition(t *testing.T, binding *v1beta1.ServiceBinding, conditionType v1beta1.ServiceBindingConditionType, status v1beta1.ConditionStatus, reason ...string) {
	foundCondition := false
	for _, condition := range binding.Status.Conditions {
		if condition.Type == conditionType {
			foundCondition = true
			if condition.Status != status {
				t.Fatalf("%v condition had unexpected status; expected %v, got %v", conditionType, status, condition.Status)
			}
			if len(reason) == 1 && condition.Reason != reason[0] {
				t.Fatalf("unexpected reason; expected %v, got %v", reason[0], condition.Reason)
			}
		}
	}

	if !foundCondition {
		t.Fatalf("%v condition not found", conditionType)
	}
}

// AssertServiceInstanceConditionFalseOrAbsent asserts that the instance's status
// either contains the given condition type with a status of False or does not
// contain the given condition.
func AssertServiceInstanceConditionFalseOrAbsent(t *testing.T, instance *v1beta1.ServiceInstance, conditionType v1beta1.ServiceInstanceConditionType) {
	for _, condition := range instance.Status.Conditions {
		if condition.Type == conditionType {
			if e, a := v1beta1.ConditionFalse, condition.Status; e != a {
				t.Fatalf("%v condition had unexpected status; expected %v, got %v", conditionType, e, a)
			}
		}
	}
}

// EnableOriginatingIdentity enables the OriginatingIdentity feature gate.  Returns
// the prior state of the gate.
func EnableOriginatingIdentity(t *testing.T, enabled bool) (previousState bool) {
	prevOrigIdentEnablement := utilfeature.DefaultMutableFeatureGate.Enabled(scfeatures.OriginatingIdentity)
	if prevOrigIdentEnablement != enabled {
		err := utilfeature.DefaultMutableFeatureGate.Set(fmt.Sprintf("%v=%v", scfeatures.OriginatingIdentity, enabled))
		if err != nil {
			t.Fatalf("Failed to enable originating identity feature: %v", err)
		}
	}
	return prevOrigIdentEnablement
}

// ContextWithUserName creates a Context with the specified userName
func ContextWithUserName(userName string) context.Context {
	ctx := genericapirequest.NewContext()
	userInfo := &user.DefaultInfo{
		Name: userName,
	}
	return genericapirequest.WithUser(ctx, userInfo)
}
