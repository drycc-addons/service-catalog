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

package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	servicecatalog "github.com/drycc-addons/service-catalog/pkg/apis/servicecatalog/v1beta1"
)

func validServiceBinding() *servicecatalog.ServiceBinding {
	return &servicecatalog.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-binding",
			Namespace: "test-ns",
		},
		Spec: servicecatalog.ServiceBindingSpec{
			InstanceRef: servicecatalog.LocalObjectReference{
				Name: "test-instance",
			},
			SecretName: "test-secret",
		},
		Status: servicecatalog.ServiceBindingStatus{
			UnbindStatus: servicecatalog.ServiceBindingUnbindStatusNotRequired,
		},
	}
}

func validServiceBindingWithInProgressBind() *servicecatalog.ServiceBinding {
	binding := validServiceBinding()
	binding.Generation = 2
	binding.Status.ReconciledGeneration = 1
	binding.Status.CurrentOperation = servicecatalog.ServiceBindingOperationBind
	now := metav1.Now()
	binding.Status.OperationStartTime = &now
	binding.Status.InProgressProperties = validServiceBindingPropertiesState()
	return binding
}

func validServiceBindingPropertiesState() *servicecatalog.ServiceBindingPropertiesState {
	return &servicecatalog.ServiceBindingPropertiesState{
		Parameters:        &runtime.RawExtension{Raw: []byte("a: 1\nb: \"2\"")},
		ParameterChecksum: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
}

func invalidServiceBindingStatusLastOperation() *string {
	runes := make([]rune, 10001)
	for i := range runes {
		runes[i] = 'a'
	}
	lastOperation := string(runes)
	return &lastOperation
}

func TestValidateServiceBinding(t *testing.T) {
	cases := []struct {
		name    string
		binding *servicecatalog.ServiceBinding
		create  bool
		valid   bool
	}{
		{
			name:    "valid",
			binding: validServiceBinding(),
			valid:   true,
		},
		{
			name: "missing namespace",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Namespace = ""
				return b
			}(),
			valid: false,
		},
		{
			name: "missing instance name",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Spec.InstanceRef.Name = ""
				return b
			}(),
			valid: false,
		},
		{
			name: "invalid instance name",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Spec.InstanceRef.Name = "test-instance-)*!"
				return b
			}(),
			valid: false,
		},
		{
			name: "missing secretName",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Spec.SecretName = ""
				return b
			}(),
			valid: false,
		},
		{
			name: "invalid secretName",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Spec.SecretName = "T_T"
				return b
			}(),
			valid: false,
		},
		{
			name: "valid parametersFrom",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Spec.ParametersFrom =
					[]servicecatalog.ParametersFromSource{
						{SecretKeyRef: &servicecatalog.SecretKeyReference{Name: "test-key-name", Key: "test-key"}}}
				return b
			}(),
			valid: true,
		},
		{
			name: "missing key reference in parametersFrom",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Spec.ParametersFrom =
					[]servicecatalog.ParametersFromSource{{SecretKeyRef: nil}}
				return b
			}(),
			valid: false,
		},
		{
			name: "key name is missing in parametersFrom",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Spec.ParametersFrom =
					[]servicecatalog.ParametersFromSource{
						{SecretKeyRef: &servicecatalog.SecretKeyReference{Name: "", Key: "test-key"}}}
				return b
			}(),
			valid: false,
		},
		{
			name: "key is missing in parametersFrom",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Spec.ParametersFrom =
					[]servicecatalog.ParametersFromSource{
						{SecretKeyRef: &servicecatalog.SecretKeyReference{Name: "test-key-name", Key: ""}}}
				return b
			}(),
			valid: false,
		},

		{
			name:    "valid with in-progress bind",
			binding: validServiceBindingWithInProgressBind(),
			valid:   true,
		},
		{
			name: "valid with in-progress unbind",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.CurrentOperation = servicecatalog.ServiceBindingOperationUnbind
				b.Status.InProgressProperties = nil
				return b
			}(),
			valid: true,
		},
		{
			name: "invalid current operation",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.CurrentOperation = servicecatalog.ServiceBindingOperation("bad-operation")
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress without updated generation",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.ReconciledGeneration = b.Generation
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress with missing OperationStartTime",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.OperationStartTime = nil
				return b
			}(),
			valid: false,
		},
		{
			name: "not in-progress with present OperationStartTime",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				now := metav1.Now()
				b.Status.OperationStartTime = &now
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress with condition ready/true",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.Conditions = []servicecatalog.ServiceBindingCondition{
					{
						Type:   servicecatalog.ServiceBindingConditionReady,
						Status: servicecatalog.ConditionTrue,
					},
				}
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress with condition ready/false",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.Conditions = []servicecatalog.ServiceBindingCondition{
					{
						Type:   servicecatalog.ServiceBindingConditionReady,
						Status: servicecatalog.ConditionFalse,
					},
				}
				return b
			}(),
			valid: true,
		},
		{
			name: "in-progress bind with missing InProgressParameters",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.InProgressProperties = nil
				return b
			}(),
			valid: false,
		},
		{
			name: "not in-progress with present InProgressParameters",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.InProgressProperties = validServiceBindingPropertiesState()
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress unbind with present InProgressParameters",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.CurrentOperation = servicecatalog.ServiceBindingOperationUnbind
				return b
			}(),
			valid: false,
		},
		{
			name: "valid in-progress properties with no parameters",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.InProgressProperties.Parameters = nil
				b.Status.InProgressProperties.ParameterChecksum = ""
				return b
			}(),
			valid: true,
		},
		{
			name: "in-progress properties parameters with missing parameters checksum",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.InProgressProperties.ParameterChecksum = ""
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters checksum with missing parameters",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.InProgressProperties.Parameters = nil
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters with missing raw",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.InProgressProperties.Parameters.Raw = []byte{}
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters with malformed yaml",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.InProgressProperties.Parameters.Raw = []byte("bad yaml")
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters checksum too small",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.InProgressProperties.ParameterChecksum = "0123456"
				return b
			}(),
			valid: false,
		},
		{
			name: "in-progress properties parameters checksum malformed",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.InProgressProperties.ParameterChecksum = "not hex"
				return b
			}(),
			valid: false,
		},
		{
			name: "valid external properties",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.ExternalProperties = validServiceBindingPropertiesState()
				return b
			}(),
			valid: true,
		},
		{
			name: "valid external properties with no parameters",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.ExternalProperties = validServiceBindingPropertiesState()
				b.Status.ExternalProperties.Parameters = nil
				b.Status.ExternalProperties.ParameterChecksum = ""
				return b
			}(),
			valid: true,
		},
		{
			name: "external properties parameters with missing parameters checksum",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.ExternalProperties = validServiceBindingPropertiesState()
				b.Status.ExternalProperties.ParameterChecksum = ""
				return b
			}(),
			valid: false,
		},
		{
			name: "external properties parameters checksum with missing parameters",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.ExternalProperties = validServiceBindingPropertiesState()
				b.Status.ExternalProperties.Parameters = nil
				return b
			}(),
			valid: false,
		},
		{
			name: "external properties parameters with missing raw",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.ExternalProperties = validServiceBindingPropertiesState()
				b.Status.ExternalProperties.Parameters.Raw = []byte{}
				return b
			}(),
			valid: false,
		},
		{
			name: "external properties parameters with malformed yaml",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.ExternalProperties = validServiceBindingPropertiesState()
				b.Status.ExternalProperties.Parameters.Raw = []byte("bad yaml")
				return b
			}(),
			valid: false,
		},
		{
			name: "external properties parameters checksum too small",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.ExternalProperties = validServiceBindingPropertiesState()
				b.Status.ExternalProperties.ParameterChecksum = "0123456"
				return b
			}(),
			valid: false,
		},
		{
			name: "external properties parameters checksum malformed",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.ExternalProperties = validServiceBindingPropertiesState()
				b.Status.ExternalProperties.ParameterChecksum = "not hex"
				return b
			}(),
			valid: false,
		},
		{
			name: "valid create",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Generation = 1
				b.Status.ReconciledGeneration = 0
				return b
			}(),
			create: true,
			valid:  true,
		},
		{
			name: "create with operation in-progress",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Generation = 1
				b.Status.ReconciledGeneration = 0
				b.Status.CurrentOperation = servicecatalog.ServiceBindingOperationBind
				return b
			}(),
			create: true,
			valid:  false,
		},
		{
			name: "create with invalid reconciled generation",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Generation = 1
				b.Status.ReconciledGeneration = 1
				return b
			}(),
			create: true,
			valid:  false,
		},
		{
			name: "update with invalid reconciled generation",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Generation = 1
				b.Status.ReconciledGeneration = 2
				return b
			}(),
			create: false,
			valid:  false,
		},
		{
			name: "failed bind starting orphan mitigation",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.OperationStartTime = nil
				b.Status.OrphanMitigationInProgress = true
				b.Status.Conditions = []servicecatalog.ServiceBindingCondition{
					{
						Type:   servicecatalog.ServiceBindingConditionReady,
						Status: servicecatalog.ConditionFalse,
					},
					{
						Type:   servicecatalog.ServiceBindingConditionFailed,
						Status: servicecatalog.ConditionTrue,
					},
				}
				return b
			}(),
			valid: true,
		},
		{
			name: "in-progress orphan mitigation",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.OrphanMitigationInProgress = true
				b.Status.Conditions = []servicecatalog.ServiceBindingCondition{
					{
						Type:   servicecatalog.ServiceBindingConditionReady,
						Status: servicecatalog.ConditionFalse,
					},
					{
						Type:   servicecatalog.ServiceBindingConditionFailed,
						Status: servicecatalog.ConditionTrue,
					},
				}
				return b
			}(),
			valid: true,
		},
		{
			name: "required unbind status on create",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.UnbindStatus = servicecatalog.ServiceBindingUnbindStatusRequired
				return b
			}(),
			create: true,
			valid:  false,
		},
		{
			name: "succeeded unbind status on create",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.UnbindStatus = servicecatalog.ServiceBindingUnbindStatusSucceeded
				return b
			}(),
			create: true,
			valid:  false,
		},
		{
			name: "failed unbind status on create",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.UnbindStatus = servicecatalog.ServiceBindingUnbindStatusFailed
				return b
			}(),
			create: true,
			valid:  false,
		},
		{
			name: "invalid unbind status on update",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.UnbindStatus = servicecatalog.ServiceBindingUnbindStatus("bad-unbind-status")
				return b
			}(),
			valid: false,
		},
		{
			name: "required unbind status on update",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.UnbindStatus = servicecatalog.ServiceBindingUnbindStatusRequired
				return b
			}(),
			valid: true,
		},
		{
			name: "succeeded unbind status on update",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.UnbindStatus = servicecatalog.ServiceBindingUnbindStatusSucceeded
				return b
			}(),
			valid: true,
		},
		{
			name: "failed unbind status on update",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBinding()
				b.Status.UnbindStatus = servicecatalog.ServiceBindingUnbindStatusFailed
				return b
			}(),
			valid: true,
		},
		{
			name: "LastOperation too long",
			binding: func() *servicecatalog.ServiceBinding {
				b := validServiceBindingWithInProgressBind()
				b.Status.LastOperation = invalidServiceBindingStatusLastOperation()
				return b
			}(),
			valid: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := internalValidateServiceBinding(tc.binding, tc.create)
			errs = append(errs, validateServiceBindingStatus(&tc.binding.Status, field.NewPath("status"), false)...)
			if len(errs) != 0 && tc.valid {
				t.Errorf("unexpected error: %v", errs)
			} else if len(errs) == 0 && !tc.valid {
				t.Error("unexpected success")
			}
		})
	}
}

func TestInternalValidateServiceBindingUpdateAllowed(t *testing.T) {
	cases := []struct {
		name              string
		newSpecChange     bool
		onGoingSpecChange bool
		valid             bool
	}{
		{
			name:              "spec change when no on-going spec change",
			newSpecChange:     true,
			onGoingSpecChange: false,
			valid:             true,
		},
		{
			name:              "spec change when on-going spec change",
			newSpecChange:     true,
			onGoingSpecChange: true,
			valid:             false,
		},
		{
			name:              "meta change when no on-going spec change",
			newSpecChange:     false,
			onGoingSpecChange: false,
			valid:             true,
		},
		{
			name:              "meta change when on-going spec change",
			newSpecChange:     false,
			onGoingSpecChange: true,
			valid:             true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oldBinding := validServiceBinding()
			if tc.onGoingSpecChange {
				oldBinding.Generation = 2
			} else {
				oldBinding.Generation = 1
			}
			oldBinding.Status.ReconciledGeneration = 1

			newBinding := validServiceBinding()
			if tc.newSpecChange {
				newBinding.Generation = oldBinding.Generation + 1
			} else {
				newBinding.Generation = oldBinding.Generation
			}
			newBinding.Status.ReconciledGeneration = 1

			errs := internalValidateServiceBindingUpdateAllowed(newBinding, oldBinding)
			if len(errs) != 0 && tc.valid {
				t.Errorf("unexpected error: %v", errs)
			} else if len(errs) == 0 && !tc.valid {
				t.Error("unexpected success")
			}
		})
	}
}
