/*
Copyright 2025 The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1beta1 "github.com/drycc-addons/service-catalog/pkg/apis/servicecatalog/v1beta1"
	servicecatalogv1beta1 "github.com/drycc-addons/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1beta1"
	gentype "k8s.io/client-go/gentype"
)

// fakeClusterServiceBrokers implements ClusterServiceBrokerInterface
type fakeClusterServiceBrokers struct {
	*gentype.FakeClientWithList[*v1beta1.ClusterServiceBroker, *v1beta1.ClusterServiceBrokerList]
	Fake *FakeServicecatalogV1beta1
}

func newFakeClusterServiceBrokers(fake *FakeServicecatalogV1beta1) servicecatalogv1beta1.ClusterServiceBrokerInterface {
	return &fakeClusterServiceBrokers{
		gentype.NewFakeClientWithList[*v1beta1.ClusterServiceBroker, *v1beta1.ClusterServiceBrokerList](
			fake.Fake,
			"",
			v1beta1.SchemeGroupVersion.WithResource("clusterservicebrokers"),
			v1beta1.SchemeGroupVersion.WithKind("ClusterServiceBroker"),
			func() *v1beta1.ClusterServiceBroker { return &v1beta1.ClusterServiceBroker{} },
			func() *v1beta1.ClusterServiceBrokerList { return &v1beta1.ClusterServiceBrokerList{} },
			func(dst, src *v1beta1.ClusterServiceBrokerList) { dst.ListMeta = src.ListMeta },
			func(list *v1beta1.ClusterServiceBrokerList) []*v1beta1.ClusterServiceBroker {
				return gentype.ToPointerSlice(list.Items)
			},
			func(list *v1beta1.ClusterServiceBrokerList, items []*v1beta1.ClusterServiceBroker) {
				list.Items = gentype.FromPointerSlice(items)
			},
		),
		fake,
	}
}
