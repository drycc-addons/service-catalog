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

package binding

import (
	"bytes"
	"strings"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/kubernetes-sigs/service-catalog/cmd/svcat/command"
	svcattest "github.com/kubernetes-sigs/service-catalog/cmd/svcat/test"
	_ "github.com/kubernetes-sigs/service-catalog/internal/test"
	"github.com/kubernetes-sigs/service-catalog/pkg/apis/servicecatalog/v1beta1"
	svcatfake "github.com/kubernetes-sigs/service-catalog/pkg/client/clientset_generated/clientset/fake"
	"github.com/kubernetes-sigs/service-catalog/pkg/svcat"
)

func TestDescribeCommand(t *testing.T) {
	const namespace = "default"
	testcases := []struct {
		name          string
		fakeBindings  []string
		bindingName   string
		expectedError string
		wantError     bool
	}{
		{
			name:          "describe non existing binding",
			fakeBindings:  []string{},
			bindingName:   "mybinding",
			expectedError: "unable to get binding '" + namespace + ".mybinding'",
			wantError:     true,
		},
		{
			name:         "describe existing binding",
			fakeBindings: []string{"mybinding"},
			bindingName:  "mybinding",
			wantError:    false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {

			// Setup fake data for the app
			k8sClient := k8sfake.NewSimpleClientset()
			var fakes []runtime.Object
			for _, name := range tc.fakeBindings {
				fakes = append(fakes, &v1beta1.ServiceBinding{
					ObjectMeta: v1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
					Spec: v1beta1.ServiceBindingSpec{},
				})
			}

			svcatClient := svcatfake.NewSimpleClientset(fakes...)
			fakeApp, _ := svcat.NewApp(k8sClient, svcatClient, namespace)
			output := &bytes.Buffer{}
			cxt := svcattest.NewContext(output, fakeApp)

			// Initialize the command arguments
			cmd := &describeCmd{
				Namespaced: command.NewNamespaced(cxt),
			}
			cmd.Namespace = namespace
			cmd.name = tc.bindingName

			err := cmd.Run()

			if tc.wantError {
				if err == nil {
					t.Errorf("expected a non-zero exit code, but the command succeeded")
				}

				errorOutput := err.Error()
				if !strings.Contains(errorOutput, tc.expectedError) {
					t.Errorf("Unexpected output:\n\nExpected:\n%q\n\nActual:\n%q\n", tc.expectedError, errorOutput)
				}
			}
			if !tc.wantError && err != nil {
				t.Errorf("expected the command to succeed but it failed with %q", err)
			}
		})
	}
}
