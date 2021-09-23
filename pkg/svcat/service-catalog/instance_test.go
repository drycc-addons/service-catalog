/*
Copyright 2018 The Kubernetes Authors.

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

package servicecatalog_test

import (
	"errors"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/testing"

	"github.com/kubernetes-sigs/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/kubernetes-sigs/service-catalog/pkg/client/clientset_generated/clientset/fake"
	. "github.com/kubernetes-sigs/service-catalog/pkg/svcat/service-catalog"
)

var _ = Describe("Instances", func() {
	var (
		sdk          *SDK
		svcCatClient *fake.Clientset
		si           *v1beta1.ServiceInstance
		si2          *v1beta1.ServiceInstance
	)

	BeforeEach(func() {
		si = &v1beta1.ServiceInstance{ObjectMeta: metav1.ObjectMeta{Name: "foobar", Namespace: "foobar_namespace"}}
		si.Status.Conditions = append(si.Status.Conditions,
			v1beta1.ServiceInstanceCondition{
				Type:   v1beta1.ServiceInstanceConditionReady,
				Status: v1beta1.ConditionTrue,
			})
		si2 = &v1beta1.ServiceInstance{ObjectMeta: metav1.ObjectMeta{Name: "barbaz", Namespace: "foobar_namespace"}}
		si2.Status.Conditions = append(si2.Status.Conditions,
			v1beta1.ServiceInstanceCondition{
				Type:   v1beta1.ServiceInstanceConditionFailed,
				Status: v1beta1.ConditionTrue,
			})
		svcCatClient = fake.NewSimpleClientset(si, si2)
		sdk = &SDK{
			ServiceCatalogClient: svcCatClient,
		}
	})
	Describe("IsInstanceFailed", func() {
		It("returns true if the Instance is in the failed status", func() {
			status := sdk.IsInstanceFailed(si2)
			Expect(status).To(BeTrue())
		})
		It("returns false if the Instance is not in the failed status", func() {
			status := sdk.IsInstanceFailed(si)
			Expect(status).To(BeFalse())
		})
	})
	Describe("IsInstanceReady", func() {
		It("returns true if the Instance is in the ready status", func() {
			status := sdk.IsInstanceReady(si)
			Expect(status).To(BeTrue())
		})
		It("returns false if the Instance is not in the ready status", func() {
			status := sdk.IsInstanceReady(si2)
			Expect(status).To(BeFalse())
		})
	})
	Describe("RetrieveInstancees", func() {
		It("Calls the generated v1beta1 List method with the specified namespace", func() {
			namespace := si.Namespace

			instances, err := sdk.RetrieveInstances(namespace, "", "")

			Expect(err).NotTo(HaveOccurred())
			Expect(instances.Items).Should(ConsistOf(*si, *si2))
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("list", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.ListActionImpl).Namespace).To(Equal(namespace))
		})
		It("Bubbles up errors", func() {
			namespace := si.Namespace
			badClient := fake.NewSimpleClientset()
			errorMessage := "error retrieving list"
			badClient.PrependReactor("list", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient

			_, err := sdk.RetrieveInstances(namespace, "", "")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errorMessage))
			Expect(badClient.Actions()[0].Matches("list", "serviceinstances")).To(BeTrue())
		})
	})
	Describe("RetrieveInstance", func() {
		It("Calls the generated v1beta1 Get method with the passed in instance", func() {
			instanceName := si.Name
			namespace := si.Namespace

			instance, err := sdk.RetrieveInstance(namespace, instanceName)
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).To(Equal(si))
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("get", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(instanceName))
			Expect(actions[0].(testing.GetActionImpl).Namespace).To(Equal(namespace))
		})
		It("Bubbles up errors", func() {
			instanceName := "not_real"
			namespace := "foobar_namespace"

			_, err := sdk.RetrieveInstance(namespace, instanceName)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("not found"))
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("get", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(instanceName))
			Expect(actions[0].(testing.GetActionImpl).Namespace).To(Equal(namespace))
		})
	})
	Describe("RetrieveInstanceByBinding", func() {
		It("Calls the generated v1beta1 Get method with the binding's namespace and the binding's instance's name", func() {
			instanceName := si.Name
			namespace := si.Namespace
			sb := &v1beta1.ServiceBinding{ObjectMeta: metav1.ObjectMeta{Name: "banana_binding", Namespace: namespace}}
			sb.Spec.InstanceRef.Name = instanceName
			instance, err := sdk.RetrieveInstanceByBinding(sb)

			Expect(err).NotTo(HaveOccurred())
			Expect(instance).NotTo(BeNil())
			Expect(instance).To(Equal(si))
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("get", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(instanceName))
			Expect(actions[0].(testing.GetActionImpl).Namespace).To(Equal(namespace))
		})
		It("Bubbles up errors", func() {
			namespace := si.Namespace
			instanceName := "not_real_instance"
			sb := &v1beta1.ServiceBinding{ObjectMeta: metav1.ObjectMeta{Name: "banana_binding", Namespace: namespace}}
			sb.Spec.InstanceRef.Name = instanceName
			badClient := &fake.Clientset{}
			errorMessage := "no instance found"
			badClient.PrependReactor("get", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient
			instance, err := sdk.RetrieveInstanceByBinding(sb)
			Expect(instance).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorMessage))
			actions := badClient.Actions()
			Expect(actions[0].Matches("get", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(instanceName))
			Expect(actions[0].(testing.GetActionImpl).Namespace).To(Equal(namespace))
		})
	})
	Describe("RetrieveInstancesByPlan", func() {
		It("Calls the generated v1beta1 List method with a ListOption containing the passed in plan", func() {
			plan := &v1beta1.ClusterServicePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_plan",
				},
				Spec: v1beta1.ClusterServicePlanSpec{},
			}
			si = &v1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar",
					Namespace: "foobar_namespace",
				},
				Spec: v1beta1.ServiceInstanceSpec{
					ClusterServicePlanRef: &v1beta1.ClusterObjectReference{
						Name: plan.Name,
					},
				},
			}
			linkedClient := fake.NewSimpleClientset(si, si2)
			sdk.ServiceCatalogClient = linkedClient

			_, err := sdk.RetrieveInstancesByPlan(plan)
			Expect(err).NotTo(HaveOccurred())
			actions := linkedClient.Actions()
			Expect(actions[0].Matches("list", "serviceinstances")).To(BeTrue())

			requirements, selectable := actions[0].(testing.ListActionImpl).GetListRestrictions().Labels.Requirements()
			Expect(selectable).Should(BeTrue())
			Expect(requirements).ShouldNot(BeEmpty())
			Expect(requirements[0].String()).To(Equal("servicecatalog.k8s.io/spec.clusterServicePlanRef.name=foobar_plan"))
		})
		It("Bubbles up errors", func() {
			badClient := fake.NewSimpleClientset()
			errorMessage := "no instances found"
			plan := &v1beta1.ClusterServicePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_plan",
				},
				Spec: v1beta1.ClusterServicePlanSpec{},
			}
			badClient.PrependReactor("list", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient

			instances, err := sdk.RetrieveInstancesByPlan(plan)
			Expect(instances).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorMessage))
			actions := badClient.Actions()
			Expect(actions[0].Matches("list", "serviceinstances")).To(BeTrue())

			requirements, selectable := actions[0].(testing.ListActionImpl).GetListRestrictions().Labels.Requirements()
			Expect(selectable).Should(BeTrue())
			Expect(requirements).ShouldNot(BeEmpty())
			Expect(requirements[0].String()).To(Equal("servicecatalog.k8s.io/spec.clusterServicePlanRef.name=foobar_plan"))
		})
	})
	Describe("UpdateInstance", func() {
		It("Properly increments the update requests field", func() {
			namespace := "cherry_namespace"
			instanceName := "cherry"
			classKubeName := "cherry_class"
			planKubeName := "cherry_plan"
			params := make(map[string]string)
			params["foo"] = "bar"
			secrets := make(map[string]string)
			secrets["username"] = "admin"
			secrets["password"] = "abc123"
			retries := 3
			opts := &ProvisionOptions{
				ExternalID: "",
				Namespace:  namespace,
				Params:     params,
				Secrets:    secrets,
			}

			provisionedInstance, err := sdk.Provision(instanceName, classKubeName, planKubeName, true, opts)
			Expect(err).To(BeNil())
			// once for the provision request
			actions := svcCatClient.Actions()
			Expect(len(actions)).To(Equal(1))

			Expect(sdk.TouchInstance(
				provisionedInstance.Namespace,
				provisionedInstance.Name,
				retries,
			)).To(BeNil())

			// verify that the get and the update happened
			actions = svcCatClient.Actions()
			// once for the get = 2
			// once for the update = 3
			Expect(len(actions)).To(Equal(3))
			getAction := actions[1]
			updateAction := actions[2]
			Expect(getAction.Matches("get", "serviceinstances")).To(BeTrue())
			Expect(updateAction.Matches("update", "serviceinstances")).To(BeTrue())

			// verify the details of the get
			get, ok := getAction.(testing.GetActionImpl)
			Expect(ok).To(BeTrue())
			Expect(get.Name).To(Equal(instanceName))
			Expect(get.Namespace).To(Equal(namespace))

			// verify the details of the update
			update, ok := updateAction.(testing.UpdateActionImpl)
			Expect(ok).To(BeTrue())
			Expect(update.Namespace).To(Equal(namespace))
			obj, ok := update.Object.(*v1beta1.ServiceInstance)
			Expect(ok).To(BeTrue())
			Expect(obj.Name).To(Equal(instanceName))
			// obj.Spec.UpdateRequests is an int64, so compare it to an int64
			Expect(obj.Spec.UpdateRequests).To(Equal(int64(1)))
		})
	})
	Describe("InstanceParentHierarchy", func() {
		It("calls the v1beta1 generated Get function repeatedly to build the hierarchy of the passed in service instance", func() {
			broker := &v1beta1.ClusterServiceBroker{ObjectMeta: metav1.ObjectMeta{Name: "foobar_broker"}}
			class := &v1beta1.ClusterServiceClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_class",
				},
				Spec: v1beta1.ClusterServiceClassSpec{
					ClusterServiceBrokerName: broker.Name,
				},
			}
			plan := &v1beta1.ClusterServicePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_plan",
				},
				Spec: v1beta1.ClusterServicePlanSpec{},
			}
			si = &v1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar",
					Namespace: "foobar_namespace",
				},
				Spec: v1beta1.ServiceInstanceSpec{
					ClusterServicePlanRef: &v1beta1.ClusterObjectReference{
						Name: plan.Name,
					},
					ClusterServiceClassRef: &v1beta1.ClusterObjectReference{
						Name: class.Name,
					},
				},
			}
			linkedClient := fake.NewSimpleClientset(si, si2, class, plan, broker)
			sdk.ServiceCatalogClient = linkedClient
			retClass, retPlan, retBroker, err := sdk.InstanceParentHierarchy(si)
			Expect(err).NotTo(HaveOccurred())
			Expect(retClass.Name).To(Equal(class.Name))
			Expect(retPlan.Name).To(Equal(plan.Name))
			Expect(retBroker.Name).To(Equal(broker.Name))
			actions := linkedClient.Actions()
			getClass := testing.GetActionImpl{
				ActionImpl: testing.ActionImpl{
					Verb: "get",
					Resource: schema.GroupVersionResource{
						Group:    "servicecatalog.k8s.io",
						Version:  "v1beta1",
						Resource: "clusterserviceclasses",
					},
				},
				Name: class.Name,
			}
			getPlan := testing.GetActionImpl{
				ActionImpl: testing.ActionImpl{
					Verb: "get",
					Resource: schema.GroupVersionResource{
						Group:    "servicecatalog.k8s.io",
						Version:  "v1beta1",
						Resource: "clusterserviceplans",
					},
				},
				Name: plan.Name,
			}
			getBroker := testing.GetActionImpl{
				ActionImpl: testing.ActionImpl{
					Verb: "get",
					Resource: schema.GroupVersionResource{
						Group:    "servicecatalog.k8s.io",
						Version:  "v1beta1",
						Resource: "clusterservicebrokers",
					},
				},
				Name: broker.Name,
			}
			Expect(actions).Should(ContainElement(getClass))
			Expect(actions).Should(ContainElement(getPlan))
			Expect(actions).Should(ContainElement(getBroker))
		})
		It("Bubbles up errors", func() {
			si = &v1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar",
					Namespace: "foobar_namespace",
				},
				Spec: v1beta1.ServiceInstanceSpec{
					ClusterServicePlanRef: &v1beta1.ClusterObjectReference{
						Name: "not_real_plan",
					},
					ClusterServiceClassRef: &v1beta1.ClusterObjectReference{
						Name: "not_real_class",
					},
				},
			}
			badClient := fake.NewSimpleClientset()
			errorMessage := "error retrieving thing"
			badClient.PrependReactor("get", "clusterserviceclasses", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			badClient.PrependReactor("get", "clusterserviceplans", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient

			a, b, c, err := sdk.InstanceParentHierarchy(si)
			Expect(a).To(BeNil())
			Expect(b).To(BeNil())
			Expect(c).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorMessage))
		})
	})
	Describe("InstanceToServiceClassAndPlan", func() {
		It("Calls the generated v1beta methods with the names of the class and plan from the passed in instance", func() {
			class := &v1beta1.ClusterServiceClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_class",
				},
			}
			plan := &v1beta1.ClusterServicePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_plan",
				},
				Spec: v1beta1.ClusterServicePlanSpec{},
			}
			si = &v1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar",
					Namespace: "foobar_namespace",
				},
				Spec: v1beta1.ServiceInstanceSpec{
					ClusterServicePlanRef: &v1beta1.ClusterObjectReference{
						Name: plan.Name,
					},
					ClusterServiceClassRef: &v1beta1.ClusterObjectReference{
						Name: class.Name,
					},
				},
			}
			linkedClient := fake.NewSimpleClientset(si, si2, class, plan)
			sdk.ServiceCatalogClient = linkedClient

			retClass, retPlan, err := sdk.InstanceToServiceClassAndPlan(si)
			Expect(err).NotTo(HaveOccurred())
			Expect(retClass).To(Equal(class))
			Expect(retPlan).To(Equal(plan))
			actions := linkedClient.Actions()
			getClass := testing.GetActionImpl{
				ActionImpl: testing.ActionImpl{
					Verb: "get",
					Resource: schema.GroupVersionResource{
						Group:    "servicecatalog.k8s.io",
						Version:  "v1beta1",
						Resource: "clusterserviceclasses",
					},
				},
				Name: class.Name,
			}
			getPlan := testing.GetActionImpl{
				ActionImpl: testing.ActionImpl{
					Verb: "get",
					Resource: schema.GroupVersionResource{
						Group:    "servicecatalog.k8s.io",
						Version:  "v1beta1",
						Resource: "clusterserviceplans",
					},
				},
				Name: plan.Name,
			}
			Expect(actions).Should(ContainElement(getClass))
			Expect(actions).Should(ContainElement(getPlan))
		})
		It("Bubbles up errors", func() {
			si = &v1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar",
					Namespace: "foobar_namespace",
				},
				Spec: v1beta1.ServiceInstanceSpec{
					ClusterServicePlanRef: &v1beta1.ClusterObjectReference{
						Name: "not_real_plan",
					},
					ClusterServiceClassRef: &v1beta1.ClusterObjectReference{
						Name: "not_real_class",
					},
				},
			}
			badClient := fake.NewSimpleClientset()
			errorMessage := "error retrieving thing"
			badClient.PrependReactor("get", "clusterserviceclasses", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			badClient.PrependReactor("get", "clusterserviceplans", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient

			a, b, err := sdk.InstanceToServiceClassAndPlan(si)
			Expect(a).To(BeNil())
			Expect(b).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorMessage))
		})
	})
	Describe("Provision", func() {
		It("Calls the v1beta1 Create method with the passed in arguments", func() {
			namespace := "cherry_namespace"
			instanceName := "cherry"
			externalID := "cherry-external-id"
			classKubeName := "cherry_class"
			planKubeName := "cherry_plan"
			params := make(map[string]string)
			params["foo"] = "bar"
			secrets := make(map[string]string)
			secrets["username"] = "admin"
			secrets["password"] = "abc123"
			opts := &ProvisionOptions{
				ExternalID: externalID,
				Namespace:  namespace,
				Params:     params,
				Secrets:    secrets,
			}

			service, err := sdk.Provision(instanceName, classKubeName, planKubeName, true, opts)

			Expect(err).NotTo(HaveOccurred())
			Expect(service.Namespace).To(Equal(namespace))
			Expect(service.Name).To(Equal(instanceName))
			Expect(service.Spec.PlanReference.ClusterServiceClassName).To(Equal(classKubeName))
			Expect(service.Spec.PlanReference.ClusterServicePlanName).To(Equal(planKubeName))
			Expect(service.Spec.ExternalID).To(Equal(externalID))

			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("create", "serviceinstances")).To(BeTrue())
			objectFromRequest := actions[0].(testing.CreateActionImpl).Object.(*v1beta1.ServiceInstance)
			Expect(objectFromRequest.ObjectMeta.Name).To(Equal(instanceName))
			Expect(objectFromRequest.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(objectFromRequest.Spec.PlanReference.ClusterServiceClassName).To(Equal(classKubeName))
			Expect(objectFromRequest.Spec.PlanReference.ClusterServicePlanName).To(Equal(planKubeName))
			Expect(objectFromRequest.Spec.Parameters.Raw).To(Equal([]byte("{\"foo\":\"bar\"}")))
			param := v1beta1.ParametersFromSource{
				SecretKeyRef: &v1beta1.SecretKeyReference{
					Name: "username",
					Key:  "admin",
				},
			}
			param2 := v1beta1.ParametersFromSource{
				SecretKeyRef: &v1beta1.SecretKeyReference{
					Name: "password",
					Key:  "abc123",
				},
			}
			Expect(objectFromRequest.Spec.ParametersFrom).Should(ConsistOf(param, param2))
			Expect(objectFromRequest.Spec.ExternalID).To(Equal(externalID))
		})
		It("Bubbles up errors", func() {
			errorMessage := "error retrieving list"
			namespace := "cherry_namespace"
			instanceName := "cherry"
			classKubeName := "cherry_class"
			planKubeName := "cherry_plan"
			params := make(map[string]string)
			params["foo"] = "bar"
			secrets := make(map[string]string)
			secrets["username"] = "admin"
			secrets["password"] = "abc123"
			badClient := fake.NewSimpleClientset()
			badClient.PrependReactor("create", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient
			opts := &ProvisionOptions{
				ExternalID: "",
				Namespace:  namespace,
				Params:     params,
				Secrets:    secrets,
			}

			service, err := sdk.Provision(instanceName, classKubeName, planKubeName, true, opts)
			Expect(service).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorMessage))
		})
	})
	Describe("Deprovision", func() {
		It("Calls the v1beta1 Delete method with the passed in service instance name", func() {
			err := sdk.Deprovision(si.Namespace, si.Name)
			Expect(err).NotTo(HaveOccurred())
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("delete", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.DeleteActionImpl).Name).To(Equal(si.Name))
		})
	})
	It("Bubbles up errors", func() {
		errorMessage := "instance not found"
		badClient := fake.NewSimpleClientset()
		badClient.PrependReactor("delete", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf(errorMessage)
		})
		sdk.ServiceCatalogClient = badClient

		err := sdk.Deprovision(si.Namespace, si.Name)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(errorMessage))
		actions := badClient.Actions()
		Expect(actions[0].Matches("delete", "serviceinstances")).To(BeTrue())
		Expect(actions[0].(testing.DeleteActionImpl).Name).To(Equal(si.Name))
	})
	Describe("WaitForInstance", func() {
		var (
			counter          int
			interval         time.Duration
			notReady         v1beta1.ServiceInstanceCondition
			notReadyInstance *v1beta1.ServiceInstance
			timeout          time.Duration
			waitClient       *fake.Clientset
		)
		BeforeEach(func() {
			counter = 0
			interval = 100 * time.Millisecond
			notReady = v1beta1.ServiceInstanceCondition{Type: v1beta1.ServiceInstanceConditionReady, Status: v1beta1.ConditionFalse}
			notReadyInstance = &v1beta1.ServiceInstance{ObjectMeta: metav1.ObjectMeta{Name: si.Name}}
			notReadyInstance.Status.Conditions = []v1beta1.ServiceInstanceCondition{notReady}
			timeout = 1 * time.Second
			waitClient = fake.NewSimpleClientset()
			waitClient.PrependReactor("get", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				counter++
				return true, notReadyInstance, nil
			})
			sdk.ServiceCatalogClient = waitClient
		})
		It("Calls the v1beta1 get instance method with the passed in name until it reaches a ready state", func() {
			waitClient.PrependReactor("get", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				if counter > 5 {
					return true, si, nil
				}
				return false, nil, nil
			})
			instance, err := sdk.WaitForInstance(si.Namespace, si.Name, interval, &timeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).ToNot(BeNil())
			Expect(instance).To(Equal(si))
			actions := waitClient.Actions()
			Expect(len(actions)).Should(BeNumerically(">", 1))
			for _, v := range actions {
				Expect(v.Matches("get", "serviceinstances")).To(BeTrue())
				Expect(v.(testing.GetActionImpl).Name).To(Equal(si.Name))
				Expect(v.(testing.GetActionImpl).Namespace).To(Equal(si.Namespace))
			}
		})
		It("Waits until the instance is Failed", func() {
			failedInstance := &v1beta1.ServiceInstance{ObjectMeta: metav1.ObjectMeta{Name: si.Name}}
			failed := v1beta1.ServiceInstanceCondition{Type: v1beta1.ServiceInstanceConditionFailed, Status: v1beta1.ConditionTrue}
			failedInstance.Status.Conditions = []v1beta1.ServiceInstanceCondition{failed}
			waitClient.PrependReactor("get", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				if counter > 5 {
					return true, failedInstance, nil
				}
				return false, nil, nil
			})
			instance, err := sdk.WaitForInstance(si.Namespace, si.Name, interval, &timeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).ToNot(BeNil())
			Expect(instance).To(Equal(failedInstance))
			actions := waitClient.Actions()
			Expect(len(actions)).Should(BeNumerically(">", 1))
			for _, v := range actions {
				Expect(v.Matches("get", "serviceinstances")).To(BeTrue())
				Expect(v.(testing.GetActionImpl).Name).To(Equal(si.Name))
				Expect(v.(testing.GetActionImpl).Namespace).To(Equal(si.Namespace))
			}
		})
		It("Bubbles up errors", func() {
			errorMessage := "backend exploded"
			waitClient.PrependReactor("get", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				if counter > 5 {
					return true, nil, errors.New(errorMessage)
				}
				return false, nil, nil
			})
			instance, err := sdk.WaitForInstance(si.Namespace, si.Name, interval, &timeout)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorMessage))
			Expect(instance).To(BeNil())
			actions := waitClient.Actions()
			Expect(len(actions)).Should(BeNumerically(">", 1))
			for _, v := range actions {
				Expect(v.Matches("get", "serviceinstances")).To(BeTrue())
				Expect(v.(testing.GetActionImpl).Name).To(Equal(si.Name))
				Expect(v.(testing.GetActionImpl).Namespace).To(Equal(si.Namespace))
			}
		})
	})
	Describe("WaitForInstanceToNotExist", func() {
		var (
			counter    int
			interval   time.Duration
			timeout    time.Duration
			waitClient *fake.Clientset
		)
		BeforeEach(func() {
			counter = 0
			interval = 100 * time.Millisecond
			timeout = 1 * time.Second
			waitClient = fake.NewSimpleClientset()
			waitClient.PrependReactor("get", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				counter++
				return true, si, nil
			})
			sdk.ServiceCatalogClient = waitClient
		})
		It("Calls the v1beta1 get instance method with the passed in service instance name until the instance no longer exists", func() {
			waitClient.PrependReactor("get", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				if counter > 5 {
					return true, nil, apierrors.NewNotFound(v1beta1.Resource("serviceinstance"), "instance not found")
				}
				return false, nil, nil
			})
			instance, err := sdk.WaitForInstanceToNotExist(si.Namespace, si.Name, interval, &timeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).To(BeNil())
			actions := waitClient.Actions()
			Expect(len(actions)).Should(BeNumerically(">", 1))
			for _, v := range actions {
				Expect(v.Matches("get", "serviceinstances")).To(BeTrue())
				Expect(v.(testing.GetActionImpl).Name).To(Equal(si.Name))
				Expect(v.(testing.GetActionImpl).Namespace).To(Equal(si.Namespace))
			}
		})
		It("Times out if the instance never goes away", func() {
			instance, err := sdk.WaitForInstanceToNotExist(si.Namespace, si.Name, interval, &timeout)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("timed out"))
			Expect(instance).ToNot(BeNil())
			actions := waitClient.Actions()
			for _, v := range actions {
				Expect(v.Matches("get", "serviceinstances")).To(BeTrue())
				Expect(v.(testing.GetActionImpl).Name).To(Equal(si.Name))
				Expect(v.(testing.GetActionImpl).Namespace).To(Equal(si.Namespace))
			}
		})
		It("Bubbles up errors", func() {
			errorMessage := "error will robinson!"
			waitClient.PrependReactor("get", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				if counter > 5 {
					return true, nil, errors.New(errorMessage)
				}
				return false, nil, nil
			})
			timeout := 1 * time.Second
			instance, err := sdk.WaitForInstanceToNotExist(si.Namespace, si.Name, 1*time.Second, &timeout)
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "timed out waiting for the condition"))
			Expect(strings.Contains(err.Error(), errorMessage))
			Expect(instance).ToNot(BeNil())
			actions := waitClient.Actions()
			Expect(actions[0].Matches("get", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(si.Name))
			Expect(actions[0].(testing.GetActionImpl).Namespace).To(Equal(si.Namespace))
		})
	})

	Describe("RemoveFinalizerForInstance", func() {
		It("Calls the generated v1beta1 put method with the passed in instance", func() {
			err := sdk.RemoveFinalizerForInstance(si.Namespace, si.Name)
			Expect(err).NotTo(HaveOccurred())

			actions := svcCatClient.Actions()
			Expect(len(actions)).To(Equal(2))
			Expect(actions[0].Matches("get", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(si.Name))
			Expect(actions[0].(testing.GetActionImpl).Namespace).To(Equal(si.Namespace))
			Expect(actions[1].Matches("update", "serviceinstances")).To(BeTrue())
			Expect(actions[1].(testing.UpdateActionImpl).Object.(*v1beta1.ServiceInstance).ObjectMeta.Name).To(Equal(si.Name))
			Expect(actions[1].(testing.UpdateActionImpl).Object.(*v1beta1.ServiceInstance).ObjectMeta.Namespace).To(Equal(si.Namespace))
		})
		It("Bubbles up errors", func() {
			errorMessage := "instance not found"
			badClient := fake.NewSimpleClientset()
			badClient.PrependReactor("get", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient

			err := sdk.RemoveFinalizerForInstance("foo", "bar")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorMessage))
			actions := badClient.Actions()
			Expect(len(actions)).To(Equal(1))
			Expect(actions[0].Matches("get", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal("bar"))
			Expect(actions[0].(testing.GetActionImpl).Namespace).To(Equal("foo"))
		})
	})
})
