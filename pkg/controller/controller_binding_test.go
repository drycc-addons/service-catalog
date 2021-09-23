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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	osb "github.com/kubernetes-sigs/go-open-service-broker-client/v2"
	fakeosb "github.com/kubernetes-sigs/go-open-service-broker-client/v2/fake"
	scmeta "github.com/kubernetes-sigs/service-catalog/pkg/api/meta"
	"github.com/kubernetes-sigs/service-catalog/pkg/apis/servicecatalog/v1beta1"
	v1beta1informers "github.com/kubernetes-sigs/service-catalog/pkg/client/informers_generated/externalversions/servicecatalog/v1beta1"
	sctestutil "github.com/kubernetes-sigs/service-catalog/test/util"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	scfeatures "github.com/kubernetes-sigs/service-catalog/pkg/features"
	"github.com/kubernetes-sigs/service-catalog/test/fake"
	clientgofake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
)

// TestReconcileServiceBindingNotInitializedStatus tests reconcileBinding to ensure that
// binding Status will be initialized when it's empty.
func TestReconcileServiceBindingNotInitializedStatus(t *testing.T) {
	_, fakeServiceCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, noFakeActions())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: "test"},
		},
		Status: v1beta1.ServiceBindingStatus{},
	}

	expectedStatus := v1beta1.ServiceBindingStatus{
		Conditions:   []v1beta1.ServiceBindingCondition{},
		UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
	}

	err := reconcileServiceBinding(t, testController, binding)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 0)

	actions := fakeServiceCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedObjBinding := assertUpdateStatus(t, actions[0], binding)
	updatedBinding, ok := updatedObjBinding.(*v1beta1.ServiceBinding)
	if !ok {
		t.Fatalf("cast error: want: *v1beta1.ServiceBinding, got: %T", updatedObjBinding)
	}
	if !reflect.DeepEqual(updatedBinding.Status, expectedStatus) {
		t.Errorf("unexpected diff: %v", diff.ObjectReflectDiff(updatedBinding.Status, expectedStatus))
	}
}

// TestReconcileBindingNonExistingInstance tests reconcileBinding to ensure a
// binding fails as expected when an instance to bind to doesn't exist.
func TestReconcileServiceBindingNonExistingServiceInstance(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, noFakeActions())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testNonExistentClusterServiceClassName},
			ExternalID:  testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := reconcileServiceBinding(t, testController, binding)
	if err == nil {
		t.Fatal("binding nothere was found and it should not be found")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed because no such instance exists.
	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingErrorBeforeRequest(t, updatedServiceBinding, errorNonexistentServiceInstanceReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := warningEventBuilder(errorNonexistentServiceInstanceReason).msgf(
		"References a non-existent ServiceInstance %q",
		"/"+testNonExistentClusterServiceClassName,
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingUnresolvedClusterServiceClassReference
// tests reconcileBinding to ensure a binding fails when a ClusterServiceClassRef has not been resolved.
func TestReconcileServiceBindingUnresolvedClusterServiceClassReference(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceName, Namespace: testNamespace},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassExternalName: testNonExistentClusterServiceClassName,
				ClusterServicePlanExternalName:  testClusterServicePlanName,
			},
			ExternalID: testServiceInstanceGUID,
		},
	}
	sharedInformers.ServiceInstances().Informer().GetStore().Add(instance)
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := reconcileServiceBinding(t, testController, binding)
	if err == nil {
		t.Fatal("serviceclassref was nil and reconcile should return an error")
	}
	if !strings.Contains(err.Error(), "not been resolved yet") {
		t.Fatalf("Did not get the expected error: %s", expectedGot("not been resolved yet", err))
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingReadyFalse(t, updatedServiceBinding, errorServiceInstanceRefsUnresolved)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := warningEventBuilder(errorServiceInstanceRefsUnresolved).msgf(
		"Binding cannot begin because ClusterServiceClass and ClusterServicePlan references for ServiceInstance \"%s/%s\" have not been resolved yet",
		binding.Namespace, binding.Spec.InstanceRef.Name,
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingUnresolvedClusterServicePlanReference
// tests reconcileBinding to ensure a binding fails when a ClusterServicePlanRef has not been resolved.
func TestReconcileServiceBindingUnresolvedClusterServicePlanReference(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceName, Namespace: testNamespace},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassExternalName: testNonExistentClusterServiceClassName,
				ClusterServicePlanExternalName:  testClusterServicePlanName,
			},
			ExternalID:             testServiceInstanceGUID,
			ClusterServiceClassRef: &v1beta1.ClusterObjectReference{Name: "Some Ref"},
		},
	}
	sharedInformers.ServiceInstances().Informer().GetStore().Add(instance)
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := reconcileServiceBinding(t, testController, binding)
	if err == nil {
		t.Fatal("serviceclass nothere was found and it should not be found")
	}

	if err := checkEventContains(err.Error(), "not been resolved yet"); err != nil {
		t.Fatal(err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingReadyFalse(t, updatedServiceBinding, errorServiceInstanceRefsUnresolved)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := warningEventBuilder(errorServiceInstanceRefsUnresolved).msgf(
		"Binding cannot begin because ClusterServiceClass and ClusterServicePlan references for ServiceInstance \"%s/%s\" have not been resolved yet",
		binding.Namespace, binding.Spec.InstanceRef.Name,
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingNonExistingClusterServiceClass tests reconcileBinding to ensure a
// binding fails as expected when a serviceclass does not exist.
func TestReconcileServiceBindingNonExistingClusterServiceClass(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceName, Namespace: testNamespace},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassExternalName: testNonExistentClusterServiceClassName,
				ClusterServicePlanExternalName:  testClusterServicePlanName,
			},
			ExternalID:             testServiceInstanceGUID,
			ClusterServiceClassRef: &v1beta1.ClusterObjectReference{Name: "nosuchclassid"},
			ClusterServicePlanRef:  &v1beta1.ClusterObjectReference{Name: "nosuchplanid"},
		},
	}
	sharedInformers.ServiceInstances().Informer().GetStore().Add(instance)
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := reconcileServiceBinding(t, testController, binding)
	if err == nil {
		t.Fatal("serviceclass nothere was found and it should not be found")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	// There is one action to update to failed status because there's
	// no such service
	assertNumberOfActions(t, actions, 1)

	// There should be one action that says it failed because no such service class.
	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingReadyFalse(t, updatedServiceBinding, errorNonexistentClusterServiceClassMessage)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := warningEventBuilder(errorNonexistentClusterServiceClassMessage).msgf(
		"References a non-existent ClusterServiceClass %q - %c",
		instance.Spec.ClusterServiceClassRef.Name, instance.Spec.PlanReference,
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingWithSecretConflict tests reconcileBinding to ensure a
// binding with an existing secret not owned by the bindings fails as expected.
func TestReconcileServiceBindingWithSecretConflict(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	// existing Secret with nil controllerRef
	addGetSecretReaction(fakeKubeClient, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceBindingName, Namespace: testNamespace},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = assertServiceBindingBindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
	fakeCatalogClient.ClearActions()

	assertGetNamespaceAction(t, fakeKubeClient.Actions())
	fakeKubeClient.ClearActions()

	assertNumberOfBrokerActions(t, fakeClusterServiceBrokerClient.Actions(), 0)

	err := reconcileServiceBinding(t, testController, binding)
	if err == nil {
		t.Fatalf("a binding should fail to create a secret: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
		Context: testContext,
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)

	assertServiceBindingReadyFalse(t, updatedServiceBinding, errorInjectingBindResultReason)
	assertServiceBindingCurrentOperation(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind)
	assertServiceBindingOperationStartTimeSet(t, updatedServiceBinding, true)
	assertServiceBindingReconciledGeneration(t, updatedServiceBinding, binding.Status.ReconciledGeneration)
	assertServiceBindingInProgressPropertiesParameters(t, updatedServiceBinding, nil, "")
	// External properties are updated because the bind request with the Broker was successful
	assertServiceBindingExternalPropertiesParameters(t, updatedServiceBinding, nil, "")
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 2)
	assertActionEquals(t, kubeActions[0], "get", "namespaces")
	assertActionEquals(t, kubeActions[1], "get", "secrets")

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := warningEventBuilder(errorInjectingBindResultReason)

	if err := checkEventPrefixes(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingWithParameters tests reconcileBinding to ensure a
// binding with parameters will be passed to the broker properly.
func TestReconcileServiceBindingWithParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	addGetSecretNotFoundReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Finalizers: []string{v1beta1.FinalizerServiceCatalog},
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	parameters := bindingParameters{Name: "test-param"}
	parameters.Args = append(parameters.Args, "first-arg")
	parameters.Args = append(parameters.Args, "second-arg")
	b, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	binding.Spec.Parameters = &runtime.RawExtension{Raw: b}

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedParameters := map[string]interface{}{
		"args": []interface{}{
			"first-arg",
			"second-arg",
		},
		"name": "test-param",
	}
	expectedParametersChecksum := generateChecksumOfParametersOrFail(t, expectedParameters)

	binding = assertServiceBindingOperationInProgressWithParametersIsTheOnlyCatalogAction(t, fakeCatalogClient, binding, v1beta1.ServiceBindingOperationBind, expectedParameters, expectedParametersChecksum)
	fakeCatalogClient.ClearActions()

	assertGetNamespaceAction(t, fakeKubeClient.Actions())
	fakeKubeClient.ClearActions()

	assertNumberOfBrokerActions(t, fakeClusterServiceBrokerClient.Actions(), 0)

	err = reconcileServiceBinding(t, testController, binding)
	if err != nil {
		t.Fatalf("a valid binding should not fail: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		Parameters: map[string]interface{}{
			"args": []interface{}{
				"first-arg",
				"second-arg",
			},
			"name": "test-param",
		},
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
		Context: testContext,
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingOperationSuccessWithParameters(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, expectedParameters, expectedParametersChecksum, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 3)
	assertActionEquals(t, kubeActions[0], "get", "namespaces")
	assertActionEquals(t, kubeActions[1], "get", "secrets")
	assertActionEquals(t, kubeActions[2], "create", "secrets")

	action := kubeActions[2].(clientgotesting.CreateAction)
	actionSecret, ok := action.GetObject().(*corev1.Secret)
	if !ok {
		t.Fatal("couldn't convert secret into a corev1.Secret")
	}
	controllerRef := metav1.GetControllerOf(actionSecret)
	if controllerRef == nil || controllerRef.UID != updatedServiceBinding.UID {
		t.Fatalf("Secret is not owned by the ServiceBinding: %v", controllerRef)
	}
	if !metav1.IsControlledBy(actionSecret, updatedServiceBinding) {
		t.Fatal("Secret is not owned by the ServiceBinding")
	}
	if e, a := testServiceBindingSecretName, actionSecret.Name; e != a {
		t.Fatalf("Unexpected name of secret; %s", expectedGot(e, a))
	}
	value, ok := actionSecret.Data["a"]
	if !ok {
		t.Fatal("Didn't find secret key 'a' in created secret")
	}
	if e, a := "b", string(value); e != a {
		t.Fatalf("Unexpected value of key 'a' in created secret; %s", expectedGot(e, a))
	}
	value, ok = actionSecret.Data["c"]
	if !ok {
		t.Fatal("Didn't find secret key 'c' in created secret")
	}
	if e, a := "d", string(value); e != a {
		t.Fatalf("Unexpected value of key 'c' in created secret; %s", expectedGot(e, a))
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := normalEventBuilder(successInjectedBindResultReason).msg(successInjectedBindResultMessage)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingWithSecretTransform tests reconcileBinding to ensure a
// binding with secretTransforms performs the specified transformations.
func TestReconcileServiceBindingWithSecretTransform(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	addGetSecretNotFoundReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Finalizers: []string{v1beta1.FinalizerServiceCatalog},
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}
	binding.Spec.SecretTransforms = []v1beta1.SecretTransform{
		{
			RenameKey: &v1beta1.RenameKeyTransform{
				From: "a",
				To:   "renamedA",
			},
		},
		{
			AddKey: &v1beta1.AddKeyTransform{
				Key:   "e",
				Value: []byte("e"),
			},
		},
	}

	if err := testController.reconcileServiceBinding(binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	binding = assertServiceBindingBindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
	fakeCatalogClient.ClearActions()

	assertGetNamespaceAction(t, fakeKubeClient.Actions())
	fakeKubeClient.ClearActions()

	assertNumberOfBrokerActions(t, fakeClusterServiceBrokerClient.Actions(), 0)

	err := testController.reconcileServiceBinding(binding)
	if err != nil {
		t.Fatalf("a valid binding should not fail: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
		Context: testContext,
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingOperationSuccess(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 3)

	// first action is a get on the namespace
	// second action is a get on the secret
	action := kubeActions[2].(clientgotesting.CreateAction)
	if e, a := "secrets", action.GetResource().Resource; e != a {
		t.Fatalf("Unexpected resource on action; %s", expectedGot(e, a))
	}
	actionSecret, ok := action.GetObject().(*corev1.Secret)
	if !ok {
		t.Fatal("couldn't convert secret into a corev1.Secret")
	}
	if e, a := testServiceBindingSecretName, actionSecret.Name; e != a {
		t.Fatalf("Unexpected name of secret; %s", expectedGot(e, a))
	}
	value, ok := actionSecret.Data["renamedA"]
	if !ok {
		t.Fatal("Didn't find secret key 'renamedA' in created secret")
	}
	if e, a := "b", string(value); e != a {
		t.Fatalf("Unexpected value of key 'renamedA' in created secret; %s", expectedGot(e, a))
	}
	value, ok = actionSecret.Data["e"]
	if !ok {
		t.Fatal("Didn't find secret key 'e' in created secret")
	}
	if e, a := "e", string(value); e != a {
		t.Fatalf("Unexpected value of key 'e' in created secret; %s", expectedGot(e, a))
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := normalEventBuilder(successInjectedBindResultReason).msg(successInjectedBindResultMessage)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingNonbindableClusterServiceClass tests reconcileBinding to ensure a
// binding for an instance that references a non-bindable service class and a
// non-bindable plan fails as expected.
func TestReconcileServiceBindingNonbindableClusterServiceClass(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestNonbindableClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestNonbindableServiceInstance())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlanNonbindable())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := reconcileServiceBinding(t, testController, binding)
	if err != nil {
		t.Fatalf("binding should fail against a non-bindable ClusterServiceClass")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingFailedBeforeRequest(t, updatedServiceBinding, errorNonbindableClusterServiceClassReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)
	assertServiceBindingReconciledGeneration(t, updatedServiceBinding, binding.Generation)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorNonbindableClusterServiceClassReason).msgf(
		"References a non-bindable ClusterServiceClass (K8S: %q ExternalName: %q) and Plan (%q) combination",
		"unbindable-clusterserviceclass", "test-unbindable-clusterserviceclass", "test-unbindable-clusterserviceplan",
	).String()
	expectedEvents := []string{expectedEvent, expectedEvent}
	if err := checkEvents(events, expectedEvents); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingNonbindableClusterServiceClassBindablePlan tests reconcileBinding
// to ensure a binding for an instance that references a non-bindable service
// class and a bindable plan fails as expected.
func TestReconcileServiceBindingNonbindableClusterServiceClassBindablePlan(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	addGetSecretNotFoundReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestNonbindableClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(func() *v1beta1.ServiceInstance {
		i := getTestServiceInstanceNonbindableServiceBindablePlan()
		i.Status = v1beta1.ServiceInstanceStatus{
			Conditions: []v1beta1.ServiceInstanceCondition{
				{
					Type:   v1beta1.ServiceInstanceConditionReady,
					Status: v1beta1.ConditionTrue,
				},
			},
		}
		return i
	}())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Finalizers: []string{v1beta1.FinalizerServiceCatalog},
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = assertServiceBindingBindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
	fakeCatalogClient.ClearActions()

	assertGetNamespaceAction(t, fakeKubeClient.Actions())
	fakeKubeClient.ClearActions()

	assertNumberOfBrokerActions(t, fakeClusterServiceBrokerClient.Actions(), 0)

	err := reconcileServiceBinding(t, testController, binding)
	if err != nil {
		t.Fatalf("A bindable plan overrides the bindability of a service class: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testNonbindableClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
		Context: testContext,
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationSuccess(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 3)
	assertActionEquals(t, kubeActions[0], "get", "namespaces")
	assertActionEquals(t, kubeActions[1], "get", "secrets")
	assertActionEquals(t, kubeActions[2], "create", "secrets")

	action := kubeActions[2].(clientgotesting.CreateAction)
	actionSecret, ok := action.GetObject().(*corev1.Secret)
	if !ok {
		t.Fatal("couldn't convert secret into a corev1.Secret")
	}
	if e, a := testServiceBindingSecretName, actionSecret.Name; e != a {
		t.Fatalf("Unexpected name of secret; %s", expectedGot(e, a))
	}
	value, ok := actionSecret.Data["a"]
	if !ok {
		t.Fatal("Didn't find secret key 'a' in created secret")
	}
	if e, a := "b", string(value); e != a {
		t.Fatalf("Unexpected value of key 'a' in created secret; %s", expectedGot(e, a))
	}
	value, ok = actionSecret.Data["c"]
	if !ok {
		t.Fatal("Didn't find secret key 'c' in created secret")
	}
	if e, a := "d", string(value); e != a {
		t.Fatalf("Unexpected value of key 'c' in created secret; %s", expectedGot(e, a))
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
}

// TestReconcileBindingBindableClusterServiceClassNonbindablePlan tests reconcileBinding
// to ensure a binding for an instance that references a bindable service class
// and a non-bindable plan fails as expected.
func TestReconcileServiceBindingBindableClusterServiceClassNonbindablePlan(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceBindableServiceNonbindablePlan())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlanNonbindable())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := reconcileServiceBinding(t, testController, binding)
	if err != nil {
		t.Fatalf("binding against a nonbindable plan should fail")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingFailedBeforeRequest(t, updatedServiceBinding, errorNonbindableClusterServiceClassReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 2)

	expectedEvent := warningEventBuilder(errorNonbindableClusterServiceClassReason).msgf(
		"References a non-bindable ClusterServiceClass (K8S: %q ExternalName: %q) and Plan (%q) combination",
		"cscguid", "test-clusterserviceclass", "test-unbindable-clusterserviceplan",
	).String()
	expectedEvents := []string{
		expectedEvent,
		expectedEvent,
	}

	if err := checkEvents(events, expectedEvents); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingInstanceNotReady tests reconcileBinding to ensure a
// binding for an instance with a ready condition set to false fails as expected.
func TestReconcileServiceBindingServiceInstanceNotReady(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithClusterRefs())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	if err := reconcileServiceBinding(t, testController, binding); err == nil {
		t.Fatalf("a binding cannot be created against an instance that is not prepared")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingErrorBeforeRequest(t, updatedServiceBinding, errorServiceInstanceNotReadyReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := warningEventBuilder(errorServiceInstanceNotReadyReason).msgf(
		"Binding cannot begin because referenced ServiceInstance %q is not ready",
		"test-ns/test-instance",
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingNamespaceError tests reconcileBinding to ensure a binding
// with an invalid namespace fails as expected.
func TestReconcileServiceBindingNamespaceError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	// prepend to override the default test namespace
	fakeKubeClient.PrependReactor("get", "namespaces", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &corev1.Namespace{}, errors.New("No namespace")
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithClusterRefs()
	setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, v1beta1.ConditionTrue, "", "")
	sharedInformers.ServiceInstances().Informer().GetStore().Add(instance)

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := reconcileServiceBinding(t, testController, binding)
	if err == nil {
		t.Fatalf("ServiceBindings are namespaced. If we cannot get the namespace we cannot find the binding")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingErrorBeforeRequest(t, updatedServiceBinding, errorFindingNamespaceServiceInstanceReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := warningEventBuilder(errorFindingNamespaceServiceInstanceReason).msgf(
		"Failed to get namespace %q during binding: %s",
		"test-ns", "No namespace",
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingDelete tests reconcileBinding to ensure a binding
// deletion works as expected.
func TestReconcileServiceBindingDelete(t *testing.T) {
	cases := []struct {
		name     string
		instance *v1beta1.ServiceInstance
		binding  *v1beta1.ServiceBinding
	}{
		{
			name:     "normal binding",
			instance: getTestServiceInstanceWithRefsAndExternalProperties(),
			binding: &v1beta1.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:              testServiceBindingName,
					Namespace:         testNamespace,
					DeletionTimestamp: &metav1.Time{},
					Finalizers:        []string{v1beta1.FinalizerServiceCatalog},
					Generation:        2,
				},
				Spec: v1beta1.ServiceBindingSpec{
					InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
					ExternalID:  testServiceBindingGUID,
					SecretName:  testServiceBindingSecretName,
				},
				Status: v1beta1.ServiceBindingStatus{
					ReconciledGeneration: 1,
					ExternalProperties:   &v1beta1.ServiceBindingPropertiesState{},
					UnbindStatus:         v1beta1.ServiceBindingUnbindStatusRequired,
				},
			},
		},
		{
			name: "binding with instance pointing to non-existent plan",
			instance: &v1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceName, Namespace: testNamespace},
				Spec: v1beta1.ServiceInstanceSpec{
					ExternalID:             testServiceInstanceGUID,
					ClusterServiceClassRef: &v1beta1.ClusterObjectReference{Name: testClusterServiceClassGUID},
					ClusterServicePlanRef:  nil,
					PlanReference: v1beta1.PlanReference{
						ClusterServiceClassExternalName: testClusterServiceClassName,
						ClusterServicePlanExternalName:  testNonExistentClusterServicePlanName,
					},
				},
				Status: v1beta1.ServiceInstanceStatus{
					ExternalProperties: &v1beta1.ServiceInstancePropertiesState{
						ClusterServicePlanExternalID:   testClusterServicePlanGUID,
						ClusterServicePlanExternalName: testClusterServicePlanName,
					},
				},
			},
			binding: &v1beta1.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:              testServiceBindingName,
					Namespace:         testNamespace,
					DeletionTimestamp: &metav1.Time{},
					Finalizers:        []string{v1beta1.FinalizerServiceCatalog},
					Generation:        2,
				},
				Spec: v1beta1.ServiceBindingSpec{
					InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
					ExternalID:  testServiceBindingGUID,
					SecretName:  testServiceBindingSecretName,
				},
				Status: v1beta1.ServiceBindingStatus{
					ReconciledGeneration: 1,
					ExternalProperties:   &v1beta1.ServiceBindingPropertiesState{},
					UnbindStatus:         v1beta1.ServiceBindingUnbindStatusRequired,
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
				UnbindReaction: &fakeosb.UnbindReaction{
					Response: &osb.UnbindResponse{},
				},
			})

			sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
			sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
			sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
			sharedInformers.ServiceInstances().Informer().GetStore().Add(tc.instance)

			binding := tc.binding
			fakeCatalogClient.AddReactor("get", "servicebindings", func(action clientgotesting.Action) (bool, runtime.Object, error) {
				return true, binding, nil
			})

			if err := reconcileServiceBinding(t, testController, binding); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			binding = assertServiceBindingUnbindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
			fakeCatalogClient.ClearActions()

			assertDeleteSecretAction(t, fakeKubeClient.Actions(), binding.Spec.SecretName)
			fakeKubeClient.ClearActions()

			assertNumberOfBrokerActions(t, fakeClusterServiceBrokerClient.Actions(), 0)

			err := reconcileServiceBinding(t, testController, binding)
			if err != nil {
				t.Fatalf("%v", err)
			}

			brokerActions := fakeClusterServiceBrokerClient.Actions()
			assertNumberOfBrokerActions(t, brokerActions, 1)
			assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
				BindingID:  testServiceBindingGUID,
				InstanceID: testServiceInstanceGUID,
				ServiceID:  testClusterServiceClassGUID,
				PlanID:     testClusterServicePlanGUID,
			})

			kubeActions := fakeKubeClient.Actions()
			// The action should be deleting the secret
			assertNumberOfActions(t, kubeActions, 1)
			assertActionEquals(t, kubeActions[0], "delete", "secrets")

			deleteAction := kubeActions[0].(clientgotesting.DeleteActionImpl)
			if e, a := binding.Spec.SecretName, deleteAction.Name; e != a {
				t.Fatalf("Unexpected name of secret: %s", expectedGot(e, a))
			}

			actions := fakeCatalogClient.Actions()
			// The actions should be:
			// 0. Updating the ready condition
			// 1. Removing finalizer
			assertNumberOfActions(t, actions, 2)

			assertUpdateStatus(t, actions[0], binding)
			updatedServiceBinding := assertUpdate(t, actions[1], binding)
			assertServiceBindingOperationSuccess(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
			assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

			events := getRecordedEvents(testController)

			expectedEvent := normalEventBuilder(successUnboundReason)
			if err := checkEventPrefixes(events, expectedEvent.stringArr()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestReconcileServiceBindingDeleteUnresolvedClusterServiceClassReference
// tests reconcileBinding to ensure a binding delete succeeds when a ClusterServiceClassRef
// has not been resolved and no action has accrued for the binding.
func TestReconcileServiceBindingDeleteUnresolvedClusterServiceClassReference(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceName, Namespace: testNamespace},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassExternalName: testNonExistentClusterServiceClassName,
				ClusterServicePlanExternalName:  testClusterServicePlanName,
			},
			ExternalID: testServiceInstanceGUID,
		},
	}
	sharedInformers.ServiceInstances().Informer().GetStore().Add(instance)
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceBindingName,
			Namespace:         testNamespace,
			DeletionTimestamp: &metav1.Time{},
			Finalizers:        []string{v1beta1.FinalizerServiceCatalog},
			Generation:        1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := reconcileServiceBinding(t, testController, binding)
	if err != nil {
		t.Fatal("should have deleted the binding")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	// The actions should be:
	// 0. Updating status
	// 1. Removing finalizer
	assertNumberOfActions(t, actions, 2)
	assertUpdateStatus(t, actions[0], binding)
	assertUpdate(t, actions[1], binding)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestSetServiceBindingCondition verifies setting a condition on a binding yields
// the results as expected with respect to the changed condition and transition
// time.
func TestSetServiceBindingCondition(t *testing.T) {
	bindingWithCondition := func(condition *v1beta1.ServiceBindingCondition) *v1beta1.ServiceBinding {
		binding := getTestServiceBinding()
		binding.Status = v1beta1.ServiceBindingStatus{
			Conditions:   []v1beta1.ServiceBindingCondition{*condition},
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusRequired,
		}

		return binding
	}

	// The value of the LastTransitionTime field on conditions has to be
	// tested to ensure it is updated correctly.
	//
	// Time basis for all condition changes:
	newTs := metav1.Now()
	oldTs := metav1.NewTime(newTs.Add(-5 * time.Minute))

	// condition is a shortcut method for creating conditions with the 'old' timestamp.
	condition := func(cType v1beta1.ServiceBindingConditionType, status v1beta1.ConditionStatus, s ...string) *v1beta1.ServiceBindingCondition {
		c := &v1beta1.ServiceBindingCondition{
			Type:   cType,
			Status: status,
		}

		if len(s) > 0 {
			c.Reason = s[0]
		}

		if len(s) > 1 {
			c.Message = s[1]
		}

		// This is the expected 'before' timestamp for all conditions under
		// test.
		c.LastTransitionTime = oldTs

		return c
	}

	// shortcut methods for creating conditions of different types

	readyFalse := func() *v1beta1.ServiceBindingCondition {
		return condition(v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse, "Reason", "Message")
	}

	readyFalsef := func(reason, message string) *v1beta1.ServiceBindingCondition {
		return condition(v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse, reason, message)
	}

	readyTrue := func() *v1beta1.ServiceBindingCondition {
		return condition(v1beta1.ServiceBindingConditionReady, v1beta1.ConditionTrue, "Reason", "Message")
	}

	failedTrue := func() *v1beta1.ServiceBindingCondition {
		return condition(v1beta1.ServiceBindingConditionFailed, v1beta1.ConditionTrue, "Reason", "Message")
	}

	// withNewTs sets the LastTransitionTime to the 'new' basis time and
	// returns it.
	withNewTs := func(c *v1beta1.ServiceBindingCondition) *v1beta1.ServiceBindingCondition {
		c.LastTransitionTime = newTs
		return c
	}

	// this test works by calling setServiceBindingCondition with the input and
	// condition fields of the test case, and ensuring that afterward the
	// input (which is mutated by the setServiceBindingCondition call) is deep-equal
	// to the test case result.
	//
	// take note of where withNewTs is used when declaring the result to
	// indicate that the LastTransitionTime field on a condition should have
	// changed.
	cases := []struct {
		name      string
		input     *v1beta1.ServiceBinding
		condition *v1beta1.ServiceBindingCondition
		result    *v1beta1.ServiceBinding
	}{
		{
			name:      "new ready condition",
			input:     getTestServiceBinding(),
			condition: readyFalse(),
			result:    bindingWithCondition(withNewTs(readyFalse())),
		},
		{
			name:      "not ready -> not ready; no ts update",
			input:     bindingWithCondition(readyFalse()),
			condition: readyFalse(),
			result:    bindingWithCondition(readyFalse()),
		},
		{
			name:      "not ready -> not ready, reason and message change; no ts update",
			input:     bindingWithCondition(readyFalse()),
			condition: readyFalsef("DifferentReason", "DifferentMessage"),
			result:    bindingWithCondition(readyFalsef("DifferentReason", "DifferentMessage")),
		},
		{
			name:      "not ready -> ready",
			input:     bindingWithCondition(readyFalse()),
			condition: readyTrue(),
			result:    bindingWithCondition(withNewTs(readyTrue())),
		},
		{
			name:      "ready -> ready; no ts update",
			input:     bindingWithCondition(readyTrue()),
			condition: readyTrue(),
			result:    bindingWithCondition(readyTrue()),
		},
		{
			name:      "ready -> not ready",
			input:     bindingWithCondition(readyTrue()),
			condition: readyFalse(),
			result:    bindingWithCondition(withNewTs(readyFalse())),
		},
		{
			name:      "not ready -> not ready + failed",
			input:     bindingWithCondition(readyFalse()),
			condition: failedTrue(),
			result: func() *v1beta1.ServiceBinding {
				i := bindingWithCondition(readyFalse())
				i.Status.Conditions = append(i.Status.Conditions, *withNewTs(failedTrue()))
				return i
			}(),
		},
	}

	for _, tc := range cases {
		setServiceBindingConditionInternal(tc.input, tc.condition.Type, tc.condition.Status, tc.condition.Reason, tc.condition.Message, newTs)

		if !reflect.DeepEqual(tc.input, tc.result) {
			t.Errorf("%v: unexpected diff: %v", tc.name, diff.ObjectReflectDiff(tc.input, tc.result))
		}
	}
}

// TestReconcileServiceBindingDeleteFailedServiceBinding tests reconcileServiceBinding to ensure
// a binding with a failed status is deleted properly.
func TestReconcileServiceBindingDeleteFailedServiceBinding(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithRefsAndExternalProperties())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := getTestServiceBindingWithFailedStatus()
	binding.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	binding.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	binding.Status.ExternalProperties = &v1beta1.ServiceBindingPropertiesState{}
	binding.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusRequired

	binding.ObjectMeta.Generation = 2
	binding.Status.ReconciledGeneration = 1

	fakeCatalogClient.AddReactor("get", "servicebindings", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, binding, nil
	})

	// updateObjectReactor is used to simulate real update and return updated object,
	// without that fake client will return empty ServiceBinding struct
	fakeCatalogClient.AddReactor(updateObjectReactor("servicebindings"))

	// After first reconcile only:
	// - status should be change: ServiceBindingUnbindStatusRequired --> ServiceBindingOperationUnbind
	// - and ServiceBinding Secret should be deleted
	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	binding = assertServiceBindingUnbindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
	fakeCatalogClient.ClearActions()

	assertDeleteSecretAction(t, fakeKubeClient.Actions(), binding.Spec.SecretName)
	fakeKubeClient.ClearActions()

	assertNumberOfBrokerActions(t, fakeClusterServiceBrokerClient.Actions(), 0)

	// After second reconcile only:
	// - status should be change: ServiceBindingOperationUnbind  --> ServiceBindingUnbindStatusSucceeded
	// - ServiceBinding Secret should be deleted
	// - Unbind request should be sent to broker
	err := reconcileServiceBinding(t, testController, binding)
	if err != nil {
		t.Fatalf("%v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
	})

	// verify one kube action occurred
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "delete", resourceName: "secrets", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	deleteAction := kubeActions[0].(clientgotesting.DeleteActionImpl)
	if e, a := binding.Spec.SecretName, deleteAction.Name; e != a {
		t.Fatalf("Unexpected name of secret: %s", expectedGot(e, a))
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	assertUpdateStatus(t, actions[0], binding)
	updatedServiceBinding := assertUpdate(t, actions[1], binding)
	assertServiceBindingOperationSuccess(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := normalEventBuilder(successUnboundReason)
	if err := checkEventPrefixes(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingWithBrokerError tests reconcileBinding to ensure a
// binding request response that contains a broker error fails as expected.
func TestReconcileServiceBindingWithClusterServiceBrokerError(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
			Error: fakeosb.UnexpectedActionError(),
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = assertServiceBindingBindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
	fakeCatalogClient.ClearActions()

	err := reconcileServiceBinding(t, testController, binding)
	if err == nil {
		t.Fatal("reconcileServiceBinding should have returned an error")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingRequestRetriableError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, errorBindCallReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorBindCallReason).msgf(
		"Error creating ServiceBinding for ServiceInstance %q of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q:",
		"test-ns/test-instance", "cscguid", "test-clusterserviceclass", "test-clusterservicebroker",
	).msg("Unexpected action")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingWithBrokerHTTPError tests reconcileBindings to ensure a
// binding request response that contains a broker HTTP error fails as expected.
func TestReconcileServiceBindingWithClusterServiceBrokerHTTPError(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
			Error: fakeosb.AsyncRequiredError(),
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Finalizers: []string{v1beta1.FinalizerServiceCatalog},
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = assertServiceBindingBindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
	fakeCatalogClient.ClearActions()

	err := reconcileServiceBinding(t, testController, binding)
	if err != nil {
		t.Fatal("reconcileServiceBinding should not have returned an error")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingRequestFailingError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, errorBindCallReason, "ServiceBindingReturnedFailure", binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvents := []string{
		warningEventBuilder(errorBindCallReason).String(),
		warningEventBuilder("ServiceBindingReturnedFailure").String(),
	}

	if err := checkEventPrefixes(events, expectedEvents); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingWithFailureCondition tests reconcileServiceBinding to ensure
// no processing is done on a binding containing a failed status.
func TestReconcileServiceBindingWithFailureCondition(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := getTestServiceBindingWithFailedStatus()

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 0)

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestReconcileServiceBindingWithServiceBindingCallFailure tests reconcileServiceBinding to ensure
// a bind creation failure is handled properly.
func TestReconcileServiceBindingWithServiceBindingCallFailure(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Error: errors.New("fake creation failure"),
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := getTestServiceBinding()

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = assertServiceBindingBindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
	fakeCatalogClient.ClearActions()

	assertGetNamespaceAction(t, fakeKubeClient.Actions())
	fakeKubeClient.ClearActions()

	assertNumberOfBrokerActions(t, fakeClusterServiceBrokerClient.Actions(), 0)

	if err := reconcileServiceBinding(t, testController, binding); err == nil {
		t.Fatal("ServiceBinding creation should fail")
	}

	// verify one kube action occurred
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingRequestRetriableError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, errorBindCallReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
		Context: testContext,
	})

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorBindCallReason).msgf(
		"Error creating ServiceBinding for ServiceInstance %q of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q:",
		"test-ns/test-instance", "cscguid", "test-clusterserviceclass", "test-clusterservicebroker",
	).msg("fake creation failure")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingWithServiceBindingFailure tests reconcileServiceBinding to ensure
// a binding request that receives an error from the broker is handled properly.
func TestReconcileServiceBindingWithServiceBindingFailure(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Error: osb.HTTPStatusCodeError{
				StatusCode:   http.StatusConflict,
				ErrorMessage: strPtr("ServiceBindingExists"),
				Description:  strPtr("Service binding with the same id, for the same service instance already exists."),
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := getTestServiceBinding()

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = assertServiceBindingBindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
	fakeCatalogClient.ClearActions()

	assertGetNamespaceAction(t, fakeKubeClient.Actions())
	fakeKubeClient.ClearActions()

	assertNumberOfBrokerActions(t, fakeClusterServiceBrokerClient.Actions(), 0)

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("ServiceBinding creation should complete: %v", err)
	}

	// verify one kube action occurred
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingRequestFailingError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, errorBindCallReason, "ServiceBindingReturnedFailure", binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
		Context: testContext,
	})

	events := getRecordedEvents(testController)

	expectedEvents := []string{
		warningEventBuilder(errorBindCallReason).String(),
		warningEventBuilder("ServiceBindingReturnedFailure").String(),
	}

	if err := checkEventPrefixes(events, expectedEvents); err != nil {
		t.Fatal(err)
	}
}

// TestUpdateBindingCondition tests updateBindingCondition to ensure all status
// condition transitions on a binding work as expected.
//
// The test cases are proving:
// - a binding with no status that has status condition set to false will update
//   the transition time
// - a binding with condition false set to condition false will not update the
//   transition time
// - a binding with condition false set to condition false with a new message and
//   reason will not update the transition time
// - a binding with condition false set to condition true will update the
//   transition time
// - a binding with condition status true set to true will not update the
//   transition time
// - a binding with condition status true set to false will update the transition
//   time
func TestUpdateServiceBindingCondition(t *testing.T) {
	getTestServiceBindingWithStatus := func(status v1beta1.ConditionStatus) *v1beta1.ServiceBinding {
		instance := getTestServiceBinding()
		instance.Status = v1beta1.ServiceBindingStatus{
			Conditions: []v1beta1.ServiceBindingCondition{{
				Type:               v1beta1.ServiceBindingConditionReady,
				Status:             status,
				Message:            "message",
				LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
			}},
		}

		return instance
	}

	// Anonymous struct fields:
	// name: short description of the test
	// input: the binding to test
	// status: condition status to set for binding condition
	// reason: reason to set for binding condition
	// message: message to set for binding condition
	// transitionTimeChanged: toggle for verifying transition time was updated
	cases := []struct {
		name                  string
		input                 *v1beta1.ServiceBinding
		status                v1beta1.ConditionStatus
		reason                string
		message               string
		transitionTimeChanged bool
		expectedLastCondition string
	}{

		{
			name:                  "initially unset",
			input:                 getTestServiceBinding(),
			status:                v1beta1.ConditionFalse,
			transitionTimeChanged: true,
			expectedLastCondition: "",
		},
		{
			name:                  "not ready -> not ready",
			input:                 getTestServiceBindingWithStatus(v1beta1.ConditionFalse),
			status:                v1beta1.ConditionFalse,
			transitionTimeChanged: false,
			expectedLastCondition: "",
		},
		{
			name:                  "not ready -> not ready, message and reason change",
			input:                 getTestServiceBindingWithStatus(v1beta1.ConditionFalse),
			status:                v1beta1.ConditionFalse,
			reason:                "foo",
			message:               "bar",
			transitionTimeChanged: false,
			expectedLastCondition: "foo",
		},
		{
			name:                  "not ready -> ready",
			input:                 getTestServiceBindingWithStatus(v1beta1.ConditionFalse),
			status:                v1beta1.ConditionTrue,
			transitionTimeChanged: true,
			expectedLastCondition: "Ready",
		},
		{
			name:                  "ready -> ready",
			input:                 getTestServiceBindingWithStatus(v1beta1.ConditionTrue),
			status:                v1beta1.ConditionTrue,
			transitionTimeChanged: false,
			expectedLastCondition: "Ready",
		},
		{
			name:                  "ready -> not ready",
			input:                 getTestServiceBindingWithStatus(v1beta1.ConditionTrue),
			status:                v1beta1.ConditionFalse,
			reason:                "foo",
			transitionTimeChanged: true,
			expectedLastCondition: "foo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, fakeCatalogClient, _, testController, _ := newTestController(t, noFakeActions())

			inputClone := tc.input.DeepCopy()

			err := testController.updateServiceBindingCondition(tc.input, v1beta1.ServiceBindingConditionReady, tc.status, tc.reason, tc.message)
			if err != nil {
				t.Fatalf("%v: error updating broker condition: %v", tc.name, err)
			}

			if !reflect.DeepEqual(tc.input, inputClone) {
				t.Fatalf("%v: updating broker condition mutated input: %s", tc.name, expectedGot(inputClone, tc.input))
			}

			actions := fakeCatalogClient.Actions()
			assertNumberOfActions(t, actions, 1)

			updatedServiceBinding := assertUpdateStatus(t, actions[0], tc.input)

			updateActionObject, ok := updatedServiceBinding.(*v1beta1.ServiceBinding)
			if !ok {
				t.Fatalf("%v: couldn't convert to binding", tc.name)
			}

			if updateActionObject.Status.LastConditionState != tc.expectedLastCondition {
				t.Fatalf("LastConditionState has unexpected value. Expected: %v, got: %v", tc.expectedLastCondition, updateActionObject.Status.LastConditionState)
			}

			var initialTs metav1.Time
			if len(inputClone.Status.Conditions) != 0 {
				initialTs = inputClone.Status.Conditions[0].LastTransitionTime
			}

			if e, a := 1, len(updateActionObject.Status.Conditions); e != a {
				t.Errorf("%v: %s", tc.name, expectedGot(e, a))
			}

			outputCondition := updateActionObject.Status.Conditions[0]
			newTs := outputCondition.LastTransitionTime

			if tc.transitionTimeChanged && initialTs == newTs {
				t.Fatalf("%v: transition time didn't change when it should have", tc.name)
			} else if !tc.transitionTimeChanged && initialTs != newTs {
				t.Fatalf("%v: transition time changed when it shouldn't have", tc.name)
			}
			if e, a := tc.reason, outputCondition.Reason; e != "" && e != a {
				t.Fatalf("%v: condition reasons didn't match; %s", tc.name, expectedGot(e, a))
			}
			if e, a := tc.message, outputCondition.Message; e != "" && e != a {
				t.Fatalf("%v: condition reasons didn't match; %s", tc.name, expectedGot(e, a))
			}
		})
	}
}

// TestReconcileUnbindingWithBrokerError tests reconcileBinding to ensure an
// unbinding request response that contains a broker error fails as expected.
func TestReconcileUnbindingWithClusterServiceBrokerError(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
			Error:    fakeosb.UnexpectedActionError(),
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	t1 := metav1.NewTime(time.Now())
	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceBindingName,
			Namespace:         testNamespace,
			DeletionTimestamp: &t1,
			Generation:        1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			ExternalProperties: &v1beta1.ServiceBindingPropertiesState{},
			UnbindStatus:       v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}
	if err := scmeta.AddFinalizer(binding, v1beta1.FinalizerServiceCatalog); err != nil {
		t.Fatalf("Finalizer error: %v", err)
	}

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = assertServiceBindingUnbindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
	fakeCatalogClient.ClearActions()

	if err := reconcileServiceBinding(t, testController, binding); err == nil {
		t.Fatal("reconcileServiceBinding should have returned an error")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingRequestRetriableError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, errorUnbindCallReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorUnbindCallReason).msgf(
		"Error unbinding from ServiceInstance %q of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q:",
		"test-ns/test-instance", "cscguid", "test-clusterserviceclass", "test-clusterservicebroker",
	).msg("Unexpected action")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileUnbindingWithClusterServiceBrokerHTTPError tests reconcileBinding to ensure an
// unbinding request response that contains a broker HTTP error fails as
// expected.
func TestReconcileUnbindingWithClusterServiceBrokerHTTPError(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
			Error: osb.HTTPStatusCodeError{
				StatusCode: http.StatusGone,
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	t1 := metav1.NewTime(time.Now())
	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceBindingName,
			Namespace:         testNamespace,
			DeletionTimestamp: &t1,
			Generation:        1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			ExternalProperties: &v1beta1.ServiceBindingPropertiesState{},
			UnbindStatus:       v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}
	if err := scmeta.AddFinalizer(binding, v1beta1.FinalizerServiceCatalog); err != nil {
		t.Fatalf("Finalizer error: %v", err)
	}

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = assertServiceBindingUnbindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
	fakeCatalogClient.ClearActions()

	if err := reconcileServiceBinding(t, testController, binding); err == nil {
		t.Fatalf("reconcileServiceBinding should have returned an error")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingRequestRetriableError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, errorUnbindCallReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorUnbindCallReason).msgf(
		"Error unbinding from ServiceInstance %q of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q:",
		"test-ns/test-instance", "cscguid", "test-clusterserviceclass", "test-clusterservicebroker",
	).msg("Status: 410; ErrorMessage: <nil>; Description: <nil>; ResponseError: <nil>")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileBindingUsingOriginatingIdentity(t *testing.T) {
	for _, tc := range originatingIdentityTestCases {
		func() {
			prevOrigIDEnablement := sctestutil.EnableOriginatingIdentity(t, tc.enableOriginatingIdentity)
			defer utilfeature.DefaultMutableFeatureGate.Set(fmt.Sprintf("%v=%v", scfeatures.OriginatingIdentity, prevOrigIDEnablement))

			fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
				BindReaction: &fakeosb.BindReaction{
					Response: &osb.BindResponse{},
				},
			})

			addGetNamespaceReaction(fakeKubeClient)
			addGetSecretNotFoundReaction(fakeKubeClient)

			sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
			sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
			sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
			sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

			binding := getTestServiceBinding()
			if tc.includeUserInfo {
				binding.Spec.UserInfo = testUserInfo
			}

			if err := reconcileServiceBinding(t, testController, binding); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			binding = assertServiceBindingBindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
			fakeCatalogClient.ClearActions()

			assertGetNamespaceAction(t, fakeKubeClient.Actions())
			fakeKubeClient.ClearActions()

			assertNumberOfBrokerActions(t, fakeBrokerClient.Actions(), 0)

			err := reconcileServiceBinding(t, testController, binding)
			if err != nil {
				t.Fatalf("%v: a valid binding should not fail: %v", tc.name, err)
			}

			brokerActions := fakeBrokerClient.Actions()
			assertNumberOfBrokerActions(t, brokerActions, 1)
			actualRequest, ok := brokerActions[0].Request.(*osb.BindRequest)
			if !ok {
				t.Errorf("%v: unexpected request type; expected %T, got %T", tc.name, &osb.BindRequest{}, actualRequest)
				return
			}
			var expectedOriginatingIdentity *osb.OriginatingIdentity
			if tc.expectedOriginatingIdentity {
				expectedOriginatingIdentity = testOriginatingIdentity
			}
			assertOriginatingIdentity(t, expectedOriginatingIdentity, actualRequest.OriginatingIdentity)
		}()
	}
}

func TestReconcileBindingDeleteUsingOriginatingIdentity(t *testing.T) {
	for _, tc := range originatingIdentityTestCases {
		func() {
			prevOrigIDEnablement := sctestutil.EnableOriginatingIdentity(t, tc.enableOriginatingIdentity)
			defer utilfeature.DefaultMutableFeatureGate.Set(fmt.Sprintf("%v=%v", scfeatures.OriginatingIdentity, prevOrigIDEnablement))

			fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
				UnbindReaction: &fakeosb.UnbindReaction{
					Response: &osb.UnbindResponse{},
				},
			})

			addGetNamespaceReaction(fakeKubeClient)
			addGetSecretNotFoundReaction(fakeKubeClient)

			sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
			sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
			sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
			sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

			binding := getTestServiceBinding()
			binding.DeletionTimestamp = &metav1.Time{}
			binding.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
			if tc.includeUserInfo {
				binding.Spec.UserInfo = testUserInfo
			}

			if err := reconcileServiceBinding(t, testController, binding); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			binding = assertServiceBindingUnbindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
			fakeCatalogClient.ClearActions()

			assertDeleteSecretAction(t, fakeKubeClient.Actions(), binding.Spec.SecretName)
			fakeKubeClient.ClearActions()

			assertNumberOfBrokerActions(t, fakeBrokerClient.Actions(), 0)

			err := reconcileServiceBinding(t, testController, binding)
			if err != nil {
				t.Fatalf("%v: a valid binding should not fail: %v", tc.name, err)
			}

			brokerActions := fakeBrokerClient.Actions()
			assertNumberOfBrokerActions(t, brokerActions, 1)
			actualRequest, ok := brokerActions[0].Request.(*osb.UnbindRequest)
			if !ok {
				t.Errorf("%v: unexpected request type; expected %T, got %T", tc.name, &osb.UnbindRequest{}, actualRequest)
				return
			}
			var expectedOriginatingIdentity *osb.OriginatingIdentity
			if tc.expectedOriginatingIdentity {
				expectedOriginatingIdentity = testOriginatingIdentity
			}
			assertOriginatingIdentity(t, expectedOriginatingIdentity, actualRequest.OriginatingIdentity)
		}()
	}
}

// TestReconcileBindingSuccessOnFinalRetry verifies that reconciliation can
// succeed on the last attempt before timing out of the retry loop
func TestReconcileBindingSuccessOnFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	addGetSecretNotFoundReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := getTestServiceBinding()
	binding.Status.CurrentOperation = v1beta1.ServiceBindingOperationBind
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	binding.Status.OperationStartTime = &startTime
	binding.Status.InProgressProperties = &v1beta1.ServiceBindingPropertiesState{}

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("a valid binding should not fail: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
		Context: testContext,
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingOperationSuccess(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successInjectedBindResultReason).msg(successInjectedBindResultMessage)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingFailureOnFinalRetry verifies that reconciliation
// completes in the event of an error after the retry duration elapses.
func TestReconcileBindingFailureOnFinalRetry(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
			Error: fakeosb.UnexpectedActionError(),
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := getTestServiceBinding()
	binding.Status.CurrentOperation = v1beta1.ServiceBindingOperationBind
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	binding.Status.OperationStartTime = &startTime

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("Should have return no error because the retry duration has elapsed: %v", err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingRequestFailingError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, errorBindCallReason, errorReconciliationRetryTimeoutReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEventPrefixes := []string{
		warningEventBuilder(errorBindCallReason).String(),
		warningEventBuilder(errorReconciliationRetryTimeoutReason).String(),
	}
	if err := checkEventPrefixes(events, expectedEventPrefixes); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingWithSecretConflictFailedAfterFinalRetry tests
// reconcileBinding to ensure a binding with an existing secret not owned by the
// bindings is marked as failed after the retry duration elapses.
func TestReconcileBindingWithSecretConflictFailedAfterFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	// existing Secret with nil controllerRef
	addGetSecretReaction(fakeKubeClient, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceBindingName, Namespace: testNamespace},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			CurrentOperation:     v1beta1.ServiceBindingOperationBind,
			OperationStartTime:   &startTime,
			UnbindStatus:         v1beta1.ServiceBindingUnbindStatusRequired,
			InProgressProperties: &v1beta1.ServiceBindingPropertiesState{},
		},
	}

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("reconciliation should complete since the retry duration has elapsed: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
		Context: testContext,
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)

	assertServiceBindingCondition(t, updatedServiceBinding, v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse, errorServiceBindingOrphanMitigation)
	assertServiceBindingCondition(t, updatedServiceBinding, v1beta1.ServiceBindingConditionFailed, v1beta1.ConditionTrue, errorReconciliationRetryTimeoutReason)
	assertServiceBindingStartingOrphanMitigation(t, updatedServiceBinding, binding)
	assertServiceBindingExternalPropertiesParameters(t, updatedServiceBinding, nil, "")

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 2)
	assertActionEquals(t, kubeActions[0], "get", "namespaces")
	assertActionEquals(t, kubeActions[1], "get", "secrets")

	events := getRecordedEvents(testController)

	expectedEventPrefixes := []string{
		warningEventBuilder(errorInjectingBindResultReason).String(),
		warningEventBuilder(errorReconciliationRetryTimeoutReason).String(),
		warningEventBuilder(errorServiceBindingOrphanMitigation).String(),
	}
	if err := checkEventPrefixes(events, expectedEventPrefixes); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingWithStatusUpdateError verifies that the
// reconciler returns an error when there is a conflict updating the status of
// the resource. This is an otherwise successful scenario where the update to set
// the in-progress operation fails.
func TestReconcileServiceBindingWithStatusUpdateError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	addGetNamespaceReaction(fakeKubeClient)
	addGetSecretNotFoundReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := getTestServiceBinding()

	fakeCatalogClient.AddReactor("update", "servicebindings", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("update error")
	})

	err := reconcileServiceBinding(t, testController, binding)
	if err == nil {
		t.Fatalf("expected error from but got none")
	}
	if e, a := "update error", err.Error(); e != a {
		t.Fatalf("unexpected error returned: %s", expectedGot(e, a))
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestReconcileServiceInstanceCredentailWithSecretParameters tests reconciling a
// binding that has parameters obtained from secrets.
func TestReconcileServiceBindingWithSecretParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)

	paramSecret := &corev1.Secret{
		Data: map[string][]byte{
			"param-secret-key": []byte("{\"b\":\"2\"}"),
		},
	}
	fakeKubeClient.AddReactor("get", "secrets", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		switch name := action.(clientgotesting.GetAction).GetName(); name {
		case "param-secret-name":
			return true, paramSecret, nil
		default:
			return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), name)
		}
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Finalizers: []string{v1beta1.FinalizerServiceCatalog},
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	parameters := map[string]interface{}{
		"a": "1",
	}
	b, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	binding.Spec.Parameters = &runtime.RawExtension{Raw: b}

	binding.Spec.ParametersFrom = []v1beta1.ParametersFromSource{
		{
			SecretKeyRef: &v1beta1.SecretKeyReference{
				Name: "param-secret-name",
				Key:  "param-secret-key",
			},
		},
	}

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedParameters := map[string]interface{}{
		"a": "1",
		"b": "<redacted>",
	}
	expectedParametersChecksum := generateChecksumOfParametersOrFail(t, map[string]interface{}{
		"a": "1",
		"b": "2",
	})

	binding = assertServiceBindingOperationInProgressWithParametersIsTheOnlyCatalogAction(t, fakeCatalogClient, binding, v1beta1.ServiceBindingOperationBind, expectedParameters, expectedParametersChecksum)
	fakeCatalogClient.ClearActions()

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 2)
	assertActionEquals(t, kubeActions[0], "get", "namespaces")
	assertActionEquals(t, kubeActions[1], "get", "secrets")
	fakeKubeClient.ClearActions()

	assertNumberOfBrokerActions(t, fakeClusterServiceBrokerClient.Actions(), 0)

	err = reconcileServiceBinding(t, testController, binding)
	if err != nil {
		t.Fatalf("a valid binding should not fail: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		Parameters: map[string]interface{}{
			"a": "1",
			"b": "2",
		},
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
		Context: testContext,
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationSuccessWithParameters(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, expectedParameters, expectedParametersChecksum, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	kubeActions = fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 4)
	assertActionEquals(t, kubeActions[0], "get", "namespaces")

	// second action is a get on the secret, to build the parameters
	assertActionEquals(t, kubeActions[1], "get", "secrets")
	action, ok := kubeActions[1].(clientgotesting.GetAction)
	if !ok {
		t.Fatalf("unexpected type of action: expected a GetAction, got %T", kubeActions[0])
	}
	if e, a := "param-secret-name", action.GetName(); e != a {
		t.Fatalf("Unexpected name of secret fetched: %s", expectedGot(e, a))
	}

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successInjectedBindResultReason).msg(successInjectedBindResultMessage)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}

}

// TestReconcileBindingWithSetOrphanMitigation tests
// reconcileServiceBinding to ensure a binding properly initiates
// orphan mitigation in the case of timeout or receiving certain HTTP codes.
func TestReconcileBindingWithSetOrphanMitigation(t *testing.T) {
	// Anonymous struct fields:
	// bindReactionError: the error to return from the bind attempt
	// setOrphanMitigation: flag for whether or not orphan mitigation
	//                      should be performed
	cases := []struct {
		name                string
		bindReactionError   error
		setOrphanMitigation bool
		shouldReturnError   bool
	}{
		{
			name:                "timeout error",
			bindReactionError:   testTimeoutError{},
			setOrphanMitigation: false,
			shouldReturnError:   true,
		},
		{
			name: "osb code 200",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 200,
			},
			setOrphanMitigation: false,
			shouldReturnError:   false,
		},
		{
			name: "osb code 201",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 201,
			},
			setOrphanMitigation: true,
			shouldReturnError:   false,
		},
		{
			name: "osb code 300",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 300,
			},
			setOrphanMitigation: false,
			shouldReturnError:   false,
		},
		{
			name: "osb code 400",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 400,
			},
			setOrphanMitigation: false,
			shouldReturnError:   false,
		},
		{
			name: "osb code 408",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 408,
			},
			setOrphanMitigation: false,
			shouldReturnError:   false,
		},
		{
			name: "osb code 500",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 500,
			},
			setOrphanMitigation: true,
			shouldReturnError:   false,
		},
		{
			name: "osb code 501",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 501,
			},
			setOrphanMitigation: true,
			shouldReturnError:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
				BindReaction: &fakeosb.BindReaction{
					Response: &osb.BindResponse{},
					Error:    tc.bindReactionError,
				},
			})

			addGetNamespaceReaction(fakeKubeClient)
			// existing Secret with nil controllerRef
			addGetSecretReaction(fakeKubeClient, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: testServiceBindingName, Namespace: testNamespace},
			})

			sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
			sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
			sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
			sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

			binding := &v1beta1.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       testServiceBindingName,
					Namespace:  testNamespace,
					Generation: 1,
				},
				Spec: v1beta1.ServiceBindingSpec{
					InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
					ExternalID:  testServiceBindingGUID,
					SecretName:  testServiceBindingSecretName,
				},
				Status: v1beta1.ServiceBindingStatus{
					UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
				},
			}
			startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
			binding.Status.OperationStartTime = &startTime

			if err := reconcileServiceBinding(t, testController, binding); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			binding = assertServiceBindingBindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
			assertServiceBindingReadyFalse(t, binding)
			fakeCatalogClient.ClearActions()

			assertGetNamespaceAction(t, fakeKubeClient.Actions())
			fakeKubeClient.ClearActions()

			assertNumberOfBrokerActions(t, fakeServiceBrokerClient.Actions(), 0)

			if err := reconcileServiceBinding(t, testController, binding); tc.shouldReturnError && err == nil || !tc.shouldReturnError && err != nil {
				t.Fatalf("expected to return %v from reconciliation attempt, got %v", tc.shouldReturnError, err)
			}

			brokerActions := fakeServiceBrokerClient.Actions()
			assertNumberOfBrokerActions(t, brokerActions, 1)
			assertBind(t, brokerActions[0], &osb.BindRequest{
				BindingID:  testServiceBindingGUID,
				InstanceID: testServiceInstanceGUID,
				ServiceID:  testClusterServiceClassGUID,
				PlanID:     testClusterServicePlanGUID,
				AppGUID:    strPtr(testNamespaceGUID),
				BindResource: &osb.BindResource{
					AppGUID: strPtr(testNamespaceGUID),
				},
				Context: testContext,
			})

			kubeActions := fakeKubeClient.Actions()
			assertNumberOfActions(t, kubeActions, 1)
			assertActionEquals(t, kubeActions[0], "get", "namespaces")

			actions := fakeCatalogClient.Actions()
			assertNumberOfActions(t, actions, 1)

			updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)

			assertServiceBindingExternalPropertiesNil(t, updatedServiceBinding)

			if tc.setOrphanMitigation {
				assertServiceBindingStartingOrphanMitigation(t, updatedServiceBinding, binding)
			} else {
				assertServiceBindingReadyFalse(t, updatedServiceBinding)
				assertServiceBindingCondition(t, updatedServiceBinding, v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse)
				assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)
			}
		})
	}
}

// TestReconcileBindingWithOrphanMitigationInProgress tests
// reconcileServiceBinding to ensure a binding is properly handled
// once orphan mitigation is underway.
func TestReconcileBindingWithOrphanMitigationInProgress(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	// existing Secret with nil controllerRef
	addGetSecretReaction(fakeKubeClient, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceBindingName, Namespace: testNamespace},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Finalizers: []string{v1beta1.FinalizerServiceCatalog},
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}
	binding.Status.CurrentOperation = v1beta1.ServiceBindingOperationBind
	binding.Status.OperationStartTime = nil
	binding.Status.OrphanMitigationInProgress = true

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("reconciliation should complete since the retry duration has elapsed: %v", err)
	}
	assertDeleteSecretAction(t, fakeKubeClient.Actions(), binding.Spec.SecretName)

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingCondition(t, updatedServiceBinding, v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse, "OrphanMitigationSuccessful")
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)
}

// TestReconcileBindingWithOrphanMitigationReconciliationRetryTimeOut tests
// reconcileServiceBinding to ensure a binding is properly handled
// once orphan mitigation is underway, specifically in the failure scenario of a
// time out during orphan mitigation.
func TestReconcileBindingWithOrphanMitigationReconciliationRetryTimeOut(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
			Error:    testTimeoutError{},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	// existing Secret with nil controllerRef
	addGetSecretReaction(fakeKubeClient, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceBindingName, Namespace: testNamespace},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Finalizers: []string{v1beta1.FinalizerServiceCatalog},
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			Conditions: []v1beta1.ServiceBindingCondition{
				{
					Type:   v1beta1.ServiceBindingConditionFailed,
					Status: v1beta1.ConditionTrue,
					Reason: "reason-orphan-mitigation-began",
				},
			},
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	binding.Status.CurrentOperation = v1beta1.ServiceBindingOperationBind
	binding.Status.OperationStartTime = &startTime
	binding.Status.OrphanMitigationInProgress = true

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("reconciliation should complete since the retry duration has elapsed: %v", err)
	}
	assertDeleteSecretAction(t, fakeKubeClient.Actions(), binding.Spec.SecretName)

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingRequestFailingError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, errorOrphanMitigationFailedReason, "reason-orphan-mitigation-began", binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEventPrefixes := []string{
		warningEventBuilder(errorUnbindCallReason).String(),
		warningEventBuilder(errorOrphanMitigationFailedReason).String(),
	}

	if err := checkEventPrefixes(events, expectedEventPrefixes); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingDeleteDuringOngoingOperation tests deleting a
// binding that has an on-going operation.
func TestReconcileServiceBindingDeleteDuringOngoingOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithRefsAndExternalProperties())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	startTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceBindingName,
			Namespace:         testNamespace,
			DeletionTimestamp: &metav1.Time{},
			Finalizers:        []string{v1beta1.FinalizerServiceCatalog},
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			CurrentOperation:     v1beta1.ServiceBindingOperationBind,
			OperationStartTime:   &startTime,
			InProgressProperties: &v1beta1.ServiceBindingPropertiesState{},
			UnbindStatus:         v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}

	fakeCatalogClient.AddReactor("get", "servicebindings", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, binding, nil
	})

	timeOfReconciliation := metav1.Now()

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = assertServiceBindingUnbindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)

	// Verify that the operation start time was reset to Now
	if binding.Status.OperationStartTime.Before(&timeOfReconciliation) {
		t.Fatalf(
			"OperationStartTime should not be before the time that the reconciliation started. OperationStartTime=%v. timeOfReconciliation=%v",
			binding.Status.OperationStartTime,
			timeOfReconciliation,
		)
	}

	fakeCatalogClient.ClearActions()

	assertDeleteSecretAction(t, fakeKubeClient.Actions(), binding.Spec.SecretName)
	fakeKubeClient.ClearActions()

	assertNumberOfBrokerActions(t, fakeClusterServiceBrokerClient.Actions(), 0)

	err := reconcileServiceBinding(t, testController, binding)
	if err != nil {
		t.Fatalf("%v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
	})

	assertDeleteSecretAction(t, fakeKubeClient.Actions(), binding.Spec.SecretName)

	actions := fakeCatalogClient.Actions()
	// The actions should be:
	// 0. Updating the current operation
	// 1. Updating the ready condition
	assertNumberOfActions(t, actions, 2)

	assertUpdateStatus(t, actions[0], binding)
	updatedServiceBinding := assertUpdate(t, actions[1], binding)
	assertServiceBindingOperationSuccess(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successUnboundReason)
	if err := checkEventPrefixes(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingDeleteDuringOrphanMitigation tests deleting a
// binding that is undergoing orphan mitigation
func TestReconcileServiceBindingDeleteDuringOrphanMitigation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithRefsAndExternalProperties())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	startTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceBindingName,
			Namespace:         testNamespace,
			DeletionTimestamp: &metav1.Time{},
			Finalizers:        []string{v1beta1.FinalizerServiceCatalog},
		},
		Spec: v1beta1.ServiceBindingSpec{
			InstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:  testServiceBindingGUID,
			SecretName:  testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			CurrentOperation:           v1beta1.ServiceBindingOperationBind,
			OperationStartTime:         &startTime,
			OrphanMitigationInProgress: true,
			UnbindStatus:               v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}

	fakeCatalogClient.AddReactor("get", "servicebindings", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, binding, nil
	})

	timeOfReconciliation := metav1.Now()

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = assertServiceBindingUnbindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)

	// Verify that the operation start time was reset to Now
	if binding.Status.OperationStartTime.Before(&timeOfReconciliation) {
		t.Fatalf(
			"OperationStartTime should not be before the time that the reconciliation started. OperationStartTime=%v. timeOfReconciliation=%v",
			binding.Status.OperationStartTime,
			timeOfReconciliation,
		)
	}

	fakeCatalogClient.ClearActions()

	assertDeleteSecretAction(t, fakeKubeClient.Actions(), binding.Spec.SecretName)
	fakeKubeClient.ClearActions()

	assertNumberOfBrokerActions(t, fakeClusterServiceBrokerClient.Actions(), 0)

	err := reconcileServiceBinding(t, testController, binding)
	if err != nil {
		t.Fatalf("%v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
	})

	assertDeleteSecretAction(t, fakeKubeClient.Actions(), binding.Spec.SecretName)

	actions := fakeCatalogClient.Actions()
	// The actions should be:
	// 0. Updating status about successful deletion
	// 1. Removing finalizers
	assertNumberOfActions(t, actions, 2)

	assertUpdateStatus(t, actions[0], binding)
	updatedServiceBinding := assertUpdate(t, actions[1], binding)
	assertServiceBindingOperationSuccess(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successUnboundReason)
	if err := checkEventPrefixes(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingAsynchronousBind tests the situation where the
// controller receives an asynchronous bind response back from the broker when
// doing a bind call.
func TestReconcileServiceBindingAsynchronousBind(t *testing.T) {
	key := osb.OperationKey(testOperation)
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Async:        true,
				OperationKey: &key,
			},
		},
	})

	utilfeature.DefaultMutableFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.AsyncBindingOperations))
	defer utilfeature.DefaultMutableFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.AsyncBindingOperations))

	addGetNamespaceReaction(fakeKubeClient)
	addGetSecretNotFoundReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestBindingRetrievableClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := getTestServiceBinding()
	bindingKey := binding.Namespace + "/" + binding.Name

	if testController.bindingPollingQueue.NumRequeues(bindingKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test binding")
	}

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = assertServiceBindingBindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
	fakeCatalogClient.ClearActions()

	assertGetNamespaceAction(t, fakeKubeClient.Actions())
	fakeKubeClient.ClearActions()

	assertNumberOfBrokerActions(t, fakeServiceBrokerClient.Actions(), 0)

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("a valid binding should not fail: %v", err)
	}

	if testController.bindingPollingQueue.NumRequeues(bindingKey) != 1 {
		t.Fatalf("Expected polling queue to have a record of seeing test binding once")
	}

	// Broker actions
	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
		AcceptsIncomplete: true,
		Context:           testContext,
	})

	// Kube actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 1)
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	// Service Catalog actions
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingAsyncInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, asyncBindingReason, testOperation, binding)

	// Events
	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := corev1.EventTypeNormal + " " + asyncBindingReason + " " + asyncBindingMessage
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event, expected %v got %v", e, a)
	}
}

// TestReconcileServiceBindingAsynchronousUnbind tests the situation where the
// controller receives an asynchronous bind response back from the broker when
// doing an unbind call.
func TestReconcileServiceBindingAsynchronousUnbind(t *testing.T) {
	key := osb.OperationKey(testOperation)
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{
				Async:        true,
				OperationKey: &key,
			},
		},
	})

	utilfeature.DefaultMutableFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.AsyncBindingOperations))
	defer utilfeature.DefaultMutableFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.AsyncBindingOperations))

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestBindingRetrievableClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := getTestServiceBindingUnbinding()
	bindingKey := binding.Namespace + "/" + binding.Name

	fakeCatalogClient.AddReactor("get", "servicebindings", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, binding, nil
	})

	if testController.bindingPollingQueue.NumRequeues(bindingKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test binding")
	}

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = assertServiceBindingUnbindInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding)
	fakeCatalogClient.ClearActions()

	assertDeleteSecretAction(t, fakeKubeClient.Actions(), binding.Spec.SecretName)
	fakeKubeClient.ClearActions()

	assertNumberOfBrokerActions(t, fakeServiceBrokerClient.Actions(), 0)

	if err := reconcileServiceBinding(t, testController, binding); err != nil {
		t.Fatalf("a valid binding should not fail: %v", err)
	}

	if testController.bindingPollingQueue.NumRequeues(bindingKey) != 1 {
		t.Fatalf("Expected polling queue to have a record of seeing test binding once")
	}

	// Broker actions
	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:         testServiceBindingGUID,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
		AcceptsIncomplete: true,
	})

	// Kube actions
	assertDeleteSecretAction(t, fakeKubeClient.Actions(), binding.Spec.SecretName)

	// Service Catalog actions
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingAsyncInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, asyncUnbindingReason, testOperation, binding)

	// Events
	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := corev1.EventTypeNormal + " " + asyncUnbindingReason + " " + asyncUnbindingMessage
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event, expected %v got %v", e, a)
	}
}

func TestPollServiceBinding(t *testing.T) {
	utilfeature.DefaultMutableFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.AsyncBindingOperations))
	defer utilfeature.DefaultMutableFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.AsyncBindingOperations))

	goneError := osb.HTTPStatusCodeError{
		StatusCode: http.StatusGone,
	}

	validatePollBindingLastOperationAction := func(t *testing.T, actions []fakeosb.Action) {
		assertNumberOfBrokerActions(t, actions, 1)

		operationKey := osb.OperationKey(testOperation)
		assertPollBindingLastOperation(t, actions[0], &osb.BindingLastOperationRequest{
			InstanceID:   testServiceInstanceGUID,
			BindingID:    testServiceBindingGUID,
			ServiceID:    strPtr(testClusterServiceClassGUID),
			PlanID:       strPtr(testClusterServicePlanGUID),
			OperationKey: &operationKey,
		})
	}

	validatePollBindingLastOperationAndGetBindingActions := func(t *testing.T, actions []fakeosb.Action) {
		assertNumberOfBrokerActions(t, actions, 2)

		operationKey := osb.OperationKey(testOperation)
		assertPollBindingLastOperation(t, actions[0], &osb.BindingLastOperationRequest{
			InstanceID:   testServiceInstanceGUID,
			BindingID:    testServiceBindingGUID,
			ServiceID:    strPtr(testClusterServiceClassGUID),
			PlanID:       strPtr(testClusterServicePlanGUID),
			OperationKey: &operationKey,
		})

		assertGetBinding(t, actions[1], &osb.GetBindingRequest{
			InstanceID: testServiceInstanceGUID,
			BindingID:  testServiceBindingGUID,
		})
	}

	cases := []struct {
		name                       string
		binding                    *v1beta1.ServiceBinding
		pollReaction               *fakeosb.PollBindingLastOperationReaction
		getBindingReaction         *fakeosb.GetBindingReaction
		environmentSetupFunc       func(t *testing.T, fakeKubeClient *clientgofake.Clientset, sharedInformers v1beta1informers.Interface)
		validateBrokerActionsFunc  func(t *testing.T, actions []fakeosb.Action)
		validateKubeActionsFunc    func(t *testing.T, actions []clientgotesting.Action)
		assertPerformedActionsFunc func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding)
		shouldError                bool
		shouldFinishPolling        bool
		expectedEvents             []string
	}{
		// Bind
		{
			name:    "bind - error",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Error: fmt.Errorf("random error"),
			},
			validateBrokerActionsFunc:  validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: nil, // does not update resources
			shouldFinishPolling:        false,
			expectedEvents:             []string{corev1.EventTypeWarning + " " + errorPollingLastOperationReason + " " + "Error polling last operation: random error"},
		},
		{
			// Special test for 410, as it is treated differently in other operations
			name:    "bind - 410 Gone considered error",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Error: goneError,
			},
			validateBrokerActionsFunc:  validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: nil, // does not update resources
			shouldFinishPolling:        false,
			expectedEvents:             []string{corev1.EventTypeWarning + " " + errorPollingLastOperationReason + " " + "Error polling last operation: " + goneError.Error()},
		},
		{
			name:    "bind - in progress",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateInProgress,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingAsyncInProgress(t, updatedBinding, v1beta1.ServiceBindingOperationBind, asyncBindingReason, testOperation, originalBinding)
			},
			shouldFinishPolling: false,
			expectedEvents:      []string{corev1.EventTypeNormal + " " + asyncBindingReason + " " + "The binding is being created asynchronously (testdescr)"},
		},
		{
			name:    "bind - failed",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateFailed,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingRequestFailingError(
					t,
					updatedBinding,
					v1beta1.ServiceBindingOperationBind,
					errorBindCallReason,
					errorBindCallReason,
					originalBinding,
				)
			},
			shouldFinishPolling: true,
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorBindCallReason + " " + "Bind call failed: " + lastOperationDescription,
				corev1.EventTypeWarning + " " + errorBindCallReason + " " + "Bind call failed: " + lastOperationDescription,
			},
		},
		{
			name:    "bind - invalid state",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       "test invalid state",
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc:  validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: nil, // does not update resources
			shouldFinishPolling:        false,
			expectedEvents:             []string{}, // does not record event
		},
		{
			name:    "bind - in progress - retry duration exceeded",
			binding: getTestServiceBindingAsyncBindingRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateInProgress,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingAsyncBindRetryDurationExceeded(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorAsyncOpTimeoutReason + " " + "The asynchronous Bind operation timed out and will not be retried",
				corev1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason + " " + "Stopping reconciliation retries because too much time has elapsed",
				corev1.EventTypeWarning + " " + errorServiceBindingOrphanMitigation + " " + "Starting orphan mitigation",
			},
		},
		{
			name:    "bind - invalid state - retry duration exceeded",
			binding: getTestServiceBindingAsyncBindingRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       "test invalid state",
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingAsyncBindRetryDurationExceeded(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorAsyncOpTimeoutReason + " " + "The asynchronous Bind operation timed out and will not be retried",
				corev1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason + " " + "Stopping reconciliation retries because too much time has elapsed",
				corev1.EventTypeWarning + " " + errorServiceBindingOrphanMitigation + " " + "Starting orphan mitigation",
			},
		},
		{
			name:    "bind - operation succeeded but GET failed",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateSucceeded,
					Description: strPtr(lastOperationDescription),
				},
			},
			getBindingReaction: &fakeosb.GetBindingReaction{
				Error: fmt.Errorf("some error"),
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAndGetBindingActions,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingAsyncBindErrorAfterStateSucceeded(t, updatedBinding, errorFetchingBindingFailedReason, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorFetchingBindingFailedReason + " " + "Could not do a GET on binding resource: some error",
				corev1.EventTypeWarning + " " + errorFetchingBindingFailedReason + " " + "Could not do a GET on binding resource: some error",
				corev1.EventTypeWarning + " " + errorServiceBindingOrphanMitigation + " " + "Starting orphan mitigation",
			},
		},
		{
			name:    "bind - operation succeeded but binding injection failed",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateSucceeded,
					Description: strPtr(lastOperationDescription),
				},
			},
			getBindingReaction: &fakeosb.GetBindingReaction{
				Response: &osb.GetBindingResponse{
					Credentials: map[string]interface{}{
						"a": "b",
						"c": "d",
					},
				},
			},
			environmentSetupFunc: func(t *testing.T, fakeKubeClient *clientgofake.Clientset, sharedInformers v1beta1informers.Interface) {
				sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
				sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestBindingRetrievableClusterServiceClass())
				sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
				sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

				addGetNamespaceReaction(fakeKubeClient)
				addGetSecretReaction(fakeKubeClient, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: testServiceBindingName, Namespace: testNamespace},
				})
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAndGetBindingActions,
			validateKubeActionsFunc: func(t *testing.T, actions []clientgotesting.Action) {
				assertNumberOfActions(t, actions, 1)
				assertActionEquals(t, actions[0], "get", "secrets")
			},
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingAsyncBindErrorAfterStateSucceeded(t, updatedBinding, errorInjectingBindResultReason, originalBinding)
			},
			shouldFinishPolling: true, // should not be requeued in polling queue; will drop back to default rate limiting
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorInjectingBindResultReason + " " + `Error injecting bind results: Secret "test-ns/test-binding" is not owned by ServiceBinding, controllerRef: nil`,
				corev1.EventTypeWarning + " " + errorInjectingBindResultReason + " " + `Error injecting bind results: Secret "test-ns/test-binding" is not owned by ServiceBinding, controllerRef: nil`,
				corev1.EventTypeWarning + " " + errorServiceBindingOrphanMitigation + " " + "Starting orphan mitigation",
			},
		},
		{
			name:    "bind - succeeded",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateSucceeded,
					Description: strPtr(lastOperationDescription),
				},
			},
			getBindingReaction: &fakeosb.GetBindingReaction{
				Response: &osb.GetBindingResponse{
					Credentials: map[string]interface{}{
						"a": "b",
						"c": "d",
					},
				},
			},
			environmentSetupFunc: func(t *testing.T, fakeKubeClient *clientgofake.Clientset, sharedInformers v1beta1informers.Interface) {
				sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
				sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestBindingRetrievableClusterServiceClass())
				sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
				sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

				addGetNamespaceReaction(fakeKubeClient)
				addGetSecretNotFoundReaction(fakeKubeClient)
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAndGetBindingActions,
			validateKubeActionsFunc: func(t *testing.T, actions []clientgotesting.Action) {
				assertNumberOfActions(t, actions, 2)
				assertActionEquals(t, actions[0], "get", "secrets")
				assertActionEquals(t, actions[1], "create", "secrets")
			},
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingOperationSuccess(t, updatedBinding, v1beta1.ServiceBindingOperationBind, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeNormal + " " + successInjectedBindResultReason + " " + successInjectedBindResultMessage},
		},
		// Unbind as part of deletion
		{
			name:    "unbind - succeeded",
			binding: getTestServiceBindingAsyncUnbinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateSucceeded,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 2)
				assertUpdateStatus(t, actions[0], originalBinding)
				updatedBinding := assertUpdate(t, actions[1], originalBinding)

				assertServiceBindingOperationSuccess(t, updatedBinding, v1beta1.ServiceBindingOperationUnbind, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeNormal + " " + successUnboundReason + " " + "The binding was deleted successfully"},
		},
		{
			name:    "unbind - 410 Gone considered succeeded",
			binding: getTestServiceBindingAsyncUnbinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Error: osb.HTTPStatusCodeError{
					StatusCode: http.StatusGone,
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 2)
				assertUpdateStatus(t, actions[0], originalBinding)
				updatedBinding := assertUpdate(t, actions[1], originalBinding)

				assertServiceBindingOperationSuccess(t, updatedBinding, v1beta1.ServiceBindingOperationUnbind, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeNormal + " " + successUnboundReason + " " + "The binding was deleted successfully"},
		},
		{
			name:    "unbind - in progress",
			binding: getTestServiceBindingAsyncUnbinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateInProgress,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingAsyncInProgress(t, updatedBinding, v1beta1.ServiceBindingOperationUnbind, asyncUnbindingReason, testOperation, originalBinding)
			},
			shouldFinishPolling: false,
			expectedEvents:      []string{corev1.EventTypeNormal + " " + asyncUnbindingReason + " " + "The binding is being deleted asynchronously (testdescr)"},
		},
		{
			name:    "unbind - error",
			binding: getTestServiceBindingAsyncUnbinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Error: fmt.Errorf("random error"),
			},
			validateBrokerActionsFunc:  validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: nil, // does not update resources
			shouldFinishPolling:        false,
			expectedEvents:             []string{corev1.EventTypeWarning + " " + errorPollingLastOperationReason + " " + "Error polling last operation: random error"},
		},
		{
			name:    "unbind - failed (retries)",
			binding: getTestServiceBindingAsyncUnbinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateFailed,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingRequestRetriableError(
					t,
					updatedBinding,
					v1beta1.ServiceBindingOperationUnbind,
					errorUnbindCallReason,
					originalBinding,
				)
			},
			shouldError:         true,
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeWarning + " " + errorUnbindCallReason + " " + "Unbind call failed: " + lastOperationDescription},
		},
		{
			name:    "unbind - invalid state",
			binding: getTestServiceBindingAsyncUnbinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       "test invalid state",
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc:  validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: nil, // does not update resources
			shouldFinishPolling:        false,
			expectedEvents:             []string{}, // does not record event
		},
		{
			name:    "unbind - in progress - retry duration exceeded",
			binding: getTestServiceBindingAsyncUnbindingRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateInProgress,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingAsyncUnbindRetryDurationExceeded(
					t,
					updatedBinding,
					v1beta1.ServiceBindingOperationUnbind,
					errorAsyncOpTimeoutReason,
					errorReconciliationRetryTimeoutReason,
					originalBinding,
				)
			},
			shouldFinishPolling: true,
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorAsyncOpTimeoutReason + " " + "The asynchronous Unbind operation timed out and will not be retried",
				corev1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason + " " + "Stopping reconciliation retries because too much time has elapsed",
			},
		},
		{
			name:    "unbind - invalid state - retry duration exceeded",
			binding: getTestServiceBindingAsyncUnbindingRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       "test invalid state",
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingAsyncUnbindRetryDurationExceeded(
					t,
					updatedBinding,
					v1beta1.ServiceBindingOperationUnbind,
					errorAsyncOpTimeoutReason,
					errorReconciliationRetryTimeoutReason,
					originalBinding,
				)
			},
			shouldFinishPolling: true,
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorAsyncOpTimeoutReason + " " + "The asynchronous Unbind operation timed out and will not be retried",
				corev1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason + " " + "Stopping reconciliation retries because too much time has elapsed",
			},
		},
		{
			name:    "unbind - failed - retry duration exceeded",
			binding: getTestServiceBindingAsyncUnbindingRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateFailed,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingRequestFailingError(
					t,
					updatedBinding,
					v1beta1.ServiceBindingOperationUnbind,
					errorUnbindCallReason,
					errorReconciliationRetryTimeoutReason,
					originalBinding,
				)
			},
			shouldFinishPolling: true,
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorUnbindCallReason + " " + "Unbind call failed: " + lastOperationDescription,
				corev1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason + " " + "Stopping reconciliation retries because too much time has elapsed",
			},
		},
		// Unbind as part of orphan mitigation
		{
			name:    "orphan mitigation - succeeded",
			binding: getTestServiceBindingAsyncOrphanMitigation(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateSucceeded,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingOrphanMitigationSuccess(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeNormal + " " + successOrphanMitigationReason + " " + successOrphanMitigationMessage},
		},
		{
			name:    "orphan mitigation - 410 Gone considered succeeded",
			binding: getTestServiceBindingAsyncOrphanMitigation(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Error: osb.HTTPStatusCodeError{
					StatusCode: http.StatusGone,
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingOrphanMitigationSuccess(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeNormal + " " + successOrphanMitigationReason + " " + successOrphanMitigationMessage},
		},
		{
			name:    "orphan mitigation - in progress",
			binding: getTestServiceBindingAsyncOrphanMitigation(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateInProgress,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingAsyncInProgress(t, updatedBinding, v1beta1.ServiceBindingOperationBind, asyncUnbindingReason, testOperation, originalBinding)
			},
			shouldFinishPolling: false,
			expectedEvents:      []string{corev1.EventTypeNormal + " " + asyncUnbindingReason + " " + "The binding is being deleted asynchronously (testdescr)"},
		},
		{
			name:    "orphan mitigation - error",
			binding: getTestServiceBindingAsyncOrphanMitigation(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Error: fmt.Errorf("random error"),
			},
			validateBrokerActionsFunc:  validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: nil, // does not update resources
			shouldFinishPolling:        false,
			expectedEvents:             []string{corev1.EventTypeWarning + " " + errorPollingLastOperationReason + " " + "Error polling last operation: random error"},
		},
		{
			name:    "orphan mitigation - failed (retries)",
			binding: getTestServiceBindingAsyncOrphanMitigation(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateFailed,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingRequestRetriableOrphanMitigation(t, updatedBinding, errorUnbindCallReason, originalBinding)
			},
			shouldError:         true,
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeWarning + " " + errorUnbindCallReason + " " + "Unbind call failed: " + lastOperationDescription},
		},
		{
			name:    "orphan mitigation - invalid state",
			binding: getTestServiceBindingAsyncOrphanMitigation(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       "test invalid state",
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc:  validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: nil, // does not update resources
			shouldFinishPolling:        false,
			expectedEvents:             []string{}, // does not record event
		},
		{
			name:    "orphan mitigation - in progress - retry duration exceeded",
			binding: getTestServiceBindingAsyncOrphanMitigationRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateInProgress,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingAsyncOrphanMitigationRetryDurationExceeded(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorAsyncOpTimeoutReason + " " + "The asynchronous Unbind operation timed out and will not be retried",
				corev1.EventTypeWarning + " " + errorOrphanMitigationFailedReason + " " + "Orphan mitigation failed: Stopping reconciliation retries because too much time has elapsed",
			},
		},
		{
			name:    "orphan mitigation - invalid state - retry duration exceeded",
			binding: getTestServiceBindingAsyncOrphanMitigationRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       "test invalid state",
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingAsyncOrphanMitigationRetryDurationExceeded(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorAsyncOpTimeoutReason + " " + "The asynchronous Unbind operation timed out and will not be retried",
				corev1.EventTypeWarning + " " + errorOrphanMitigationFailedReason + " " + "Orphan mitigation failed: Stopping reconciliation retries because too much time has elapsed",
			},
		},
		{
			name:    "orphan mitigation - failed - retry duration exceeded",
			binding: getTestServiceBindingAsyncOrphanMitigationRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateFailed,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			assertPerformedActionsFunc: func(t *testing.T, actions []clientgotesting.Action, originalBinding *v1beta1.ServiceBinding) {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], originalBinding)

				assertServiceBindingAsyncOrphanMitigationRetryDurationExceeded(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorUnbindCallReason + " " + "Unbind call failed: " + lastOperationDescription,
				corev1.EventTypeWarning + " " + errorOrphanMitigationFailedReason + " " + "Orphan mitigation failed: Stopping reconciliation retries because too much time has elapsed",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
				PollBindingLastOperationReaction: tc.pollReaction,
				GetBindingReaction:               tc.getBindingReaction,
			})

			if tc.environmentSetupFunc != nil {
				tc.environmentSetupFunc(t, fakeKubeClient, sharedInformers)
			} else {
				// default
				sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
				sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestBindingRetrievableClusterServiceClass())
				sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
				sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
			}

			bindingKey := tc.binding.Namespace + "/" + tc.binding.Name

			err := testController.pollServiceBinding(tc.binding)
			if tc.shouldError && err == nil {
				t.Fatalf("expected error when polling service binding but there was none")
			} else if !tc.shouldError && err != nil {
				t.Fatalf("unexpected error when polling service binding: %v", err)
			}

			if tc.shouldFinishPolling && testController.bindingPollingQueue.NumRequeues(bindingKey) != 0 {
				t.Fatalf("Expected polling queue to not have any record of test binding as polling should have completed")
			} else if !tc.shouldFinishPolling && testController.bindingPollingQueue.NumRequeues(bindingKey) != 1 {
				t.Fatalf("Expected polling queue to have record of seeing test binding once")
			}

			// Broker actions
			brokerActions := fakeServiceBrokerClient.Actions()

			if tc.validateBrokerActionsFunc != nil {
				tc.validateBrokerActionsFunc(t, brokerActions)
			} else {
				assertNumberOfBrokerActions(t, brokerActions, 0)
			}

			// Kube actions
			kubeActions := fakeKubeClient.Actions()

			if tc.validateKubeActionsFunc != nil {
				tc.validateKubeActionsFunc(t, kubeActions)
			} else {
				assertNumberOfActions(t, kubeActions, 0)
			}

			// Catalog actions
			actions := fakeCatalogClient.Actions()
			if tc.assertPerformedActionsFunc != nil {
				tc.assertPerformedActionsFunc(t, actions, tc.binding)
			} else {
				assertNumberOfActions(t, actions, 0)
			}

			// Events
			events := getRecordedEvents(testController)
			assertNumEvents(t, events, len(tc.expectedEvents))

			for idx, expectedEvent := range tc.expectedEvents {
				if e, a := expectedEvent, events[idx]; e != a {
					t.Fatalf("Received unexpected event #%v, expected %v got %v", idx, e, a)
				}
			}
		})
	}
}

func TestTransformSecretData(t *testing.T) {
	cases := []struct {
		name                   string
		transforms             []v1beta1.SecretTransform
		credentials            map[string]interface{}
		transformedCredentials map[string]interface{}
		otherSecret            *corev1.Secret
	}{
		{
			name: "RenameKeyTransform",
			transforms: []v1beta1.SecretTransform{
				{
					RenameKey: &v1beta1.RenameKeyTransform{
						From: "foo",
						To:   "bar",
					},
				},
			},
			credentials: map[string]interface{}{
				"foo": "123",
			},
			transformedCredentials: map[string]interface{}{
				"bar": "123",
			},
		},
		{
			name: "AddKeyTransform with value",
			transforms: []v1beta1.SecretTransform{
				{
					AddKey: &v1beta1.AddKeyTransform{
						Key:   "bar",
						Value: []byte("456"),
					},
				},
			},
			credentials: map[string]interface{}{
				"foo": "123",
			},
			transformedCredentials: map[string]interface{}{
				"foo": "123",
				"bar": []byte("456"),
			},
		},
		{
			name: "AddKeyTransform with stringValue",
			transforms: []v1beta1.SecretTransform{
				{
					AddKey: &v1beta1.AddKeyTransform{
						Key:         "bar",
						StringValue: strPtr("456"),
					},
				},
			},
			credentials: map[string]interface{}{
				"foo": "123",
			},
			transformedCredentials: map[string]interface{}{
				"foo": "123",
				"bar": "456",
			},
		},
		{
			name: "AddKeyTransform with JSONPathExpression",
			transforms: []v1beta1.SecretTransform{
				{
					AddKey: &v1beta1.AddKeyTransform{
						Key:                "bar",
						JSONPathExpression: strPtr("{.foo}"),
					},
				},
			},
			credentials: map[string]interface{}{
				"foo": "123",
			},
			transformedCredentials: map[string]interface{}{
				"foo": "123",
				"bar": "123",
			},
		},
		{
			name: "AddKeyTransform with JSONPathExpression on non-flat credentials",
			transforms: []v1beta1.SecretTransform{
				{
					AddKey: &v1beta1.AddKeyTransform{
						Key:                "child-of-foo",
						JSONPathExpression: strPtr("{.foo.child}"),
					},
				},
			},
			credentials: map[string]interface{}{
				"foo": map[string]interface{}{
					"child": "123",
				},
			},
			transformedCredentials: map[string]interface{}{
				"foo": map[string]interface{}{
					"child": "123",
				},
				"child-of-foo": "123",
			},
		},
		{
			name: "AddKeyTransform stringValue precedence over value",
			transforms: []v1beta1.SecretTransform{
				{
					AddKey: &v1beta1.AddKeyTransform{
						Key:         "bar",
						Value:       []byte("456"),
						StringValue: strPtr("789"),
					},
				},
			},
			credentials: map[string]interface{}{
				"foo": "123",
			},
			transformedCredentials: map[string]interface{}{
				"foo": "123",
				"bar": "789",
			},
		},
		{
			name: "MergeSecretTransform",
			transforms: []v1beta1.SecretTransform{
				{
					AddKeysFrom: &v1beta1.AddKeysFromTransform{
						SecretRef: &v1beta1.ObjectReference{
							Namespace: "ns",
							Name:      "other-secret",
						},
					},
				},
			},
			credentials: map[string]interface{}{
				"foo": []byte("123"),
			},
			otherSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
					Name:      "other-secret",
				},
				Data: map[string][]byte{
					"bar": []byte("456"),
				},
			},
			transformedCredentials: map[string]interface{}{
				"foo": []byte("123"),
				"bar": []byte("456"),
			},
		},
		{
			name: "RemoveKeyTransform",
			transforms: []v1beta1.SecretTransform{
				{
					RemoveKey: &v1beta1.RemoveKeyTransform{
						Key: "bar",
					},
				},
			},
			credentials: map[string]interface{}{
				"foo": "123",
				"bar": "456",
			},
			transformedCredentials: map[string]interface{}{
				"foo": "123",
			},
		},
	}

	for _, tc := range cases {
		fakeKubeClient, _, _, testController, _ := newTestController(t, fakeosb.FakeClientConfiguration{})

		if tc.otherSecret != nil {
			addGetSecretReaction(fakeKubeClient, tc.otherSecret)
		}

		err := testController.transformCredentials(tc.transforms, tc.credentials)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(tc.credentials, tc.transformedCredentials) {
			t.Errorf("%v: unexpected transformed secret data; expected: %v; actual: %v", tc.name, tc.transformedCredentials, tc.credentials)
		}
	}
}

func assertServiceBindingBindInProgressIsTheOnlyCatalogAction(t *testing.T, fakeCatalogClient *fake.Clientset, binding *v1beta1.ServiceBinding) *v1beta1.ServiceBinding {
	return assertServiceBindingOperationInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding, v1beta1.ServiceBindingOperationBind)
}

func assertServiceBindingUnbindInProgressIsTheOnlyCatalogAction(t *testing.T, fakeCatalogClient *fake.Clientset, binding *v1beta1.ServiceBinding) *v1beta1.ServiceBinding {
	return assertServiceBindingOperationInProgressIsTheOnlyCatalogAction(t, fakeCatalogClient, binding, v1beta1.ServiceBindingOperationUnbind)
}

func assertServiceBindingOperationInProgressIsTheOnlyCatalogAction(t *testing.T, fakeCatalogClient *fake.Clientset, binding *v1beta1.ServiceBinding, operation v1beta1.ServiceBindingOperation) *v1beta1.ServiceBinding {
	return assertServiceBindingOperationInProgressWithParametersIsTheOnlyCatalogAction(t, fakeCatalogClient, binding, operation, nil, "")
}

func assertServiceBindingOperationInProgressWithParametersIsTheOnlyCatalogAction(t *testing.T, fakeCatalogClient *fake.Clientset, binding *v1beta1.ServiceBinding, operation v1beta1.ServiceBindingOperation, parameters map[string]interface{}, parametersChecksum string) *v1beta1.ServiceBinding {
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)
	updateObject := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationInProgressWithParameters(t, updateObject, operation, parameters, parametersChecksum, binding)
	assertServiceBindingOrphanMitigationSet(t, updateObject, false)
	return updateObject.(*v1beta1.ServiceBinding)
}

func assertGetNamespaceAction(t *testing.T, kubeActions []clientgotesting.Action) {
	assertNumberOfActions(t, kubeActions, 1)
	assertActionEquals(t, kubeActions[0], "get", "namespaces")
}

func assertDeleteSecretAction(t *testing.T, kubeActions []clientgotesting.Action, secretName string) {
	assertNumberOfActions(t, kubeActions, 1)
	assertActionEquals(t, kubeActions[0], "delete", "secrets")

	deleteAction := kubeActions[0].(clientgotesting.DeleteActionImpl)
	if e, a := secretName, deleteAction.Name; e != a {
		t.Fatalf("Unexpected name of secret: %s", expectedGot(e, a))
	}
}

func assertActionEquals(t *testing.T, action clientgotesting.Action, expectedVerb, expectedResource string) {
	if e, a := expectedVerb, action.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on action; %s", expectedGot(e, a))
	}
	if e, a := expectedResource, action.GetResource().Resource; e != a {
		t.Fatalf("Unexpected resource on action; %s", expectedGot(e, a))
	}
}

func reconcileServiceBinding(t *testing.T, testController *controller, binding *v1beta1.ServiceBinding) error {
	clone := binding.DeepCopy()
	err := testController.reconcileServiceBinding(binding)
	if !reflect.DeepEqual(binding, clone) {
		t.Errorf("reconcileServiceBinding shouldn't mutate input, but it does: %s", expectedGot(clone, binding))
	}
	return err
}
