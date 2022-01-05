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

package migration

import (
	"context"

	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// DisableBlocker deletes blocking validation webhook
func (m *Service) DisableBlocker(baseName string) {
	klog.Info("Deleting deployment of WriteBlocker")

	klog.Info("Deleting ValidatingWebhook")
	err := m.admInterface.ValidatingWebhookConfigurations().Delete(context.Background(), baseName, metav1.DeleteOptions{})
	if err != nil {
		klog.Warning(err)
	}

	klog.Info("WriteBlocker was removed")
}

// EnableBlocker creates blocking validation webhook
func (m *Service) EnableBlocker(baseName string) error {
	klog.Info("Starting deployment of WriteBlocker")

	klog.Info("Creating ValidationWebhook")
	webhookConf := getValidationWebhookConfigurationObject(baseName)
	_, err := m.admInterface.ValidatingWebhookConfigurations().Create(context.Background(), webhookConf, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	klog.Info("WriteBlocker deployment finished successfully. All Service Catalog CRDs are read only")
	return nil
}

func getValidationWebhookConfigurationObject(name string) *v1.ValidatingWebhookConfiguration {
	path := "/this-endpoint-does-not-have-to-exist"
	failurePolicy := v1.Fail

	return &v1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Webhooks: []v1.ValidatingWebhook{
			{
				Name:          "validating.reject-changes-to-service-catalog-crds.servicecatalog.k8s.io",
				FailurePolicy: &failurePolicy,
				ClientConfig: v1.WebhookClientConfig{
					Service: &v1.ServiceReference{
						Name:      name,
						Namespace: "dummy",
						Path:      &path,
					},
				},
				Rules: []v1.RuleWithOperations{
					{
						Operations: []v1.OperationType{
							v1.Create,
							v1.Update,
							v1.Delete,
						},
						Rule: v1.Rule{
							APIGroups:   []string{"servicecatalog.k8s.io"},
							APIVersions: []string{"v1beta1"},
							Resources: []string{
								"clusterservicebrokers",
								"clusterserviceclasses",
								"serviceclasses",
								"clusterserviceplans",
								"serviceplans",
								"servicebindings",
								"servicebrokers",
								"serviceinstances",
							},
						},
					},
				},
			},
		},
	}
}
