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

// fakeClusterServicePlans implements ClusterServicePlanInterface
type fakeClusterServicePlans struct {
	*gentype.FakeClientWithList[*v1beta1.ClusterServicePlan, *v1beta1.ClusterServicePlanList]
	Fake *FakeServicecatalogV1beta1
}

func newFakeClusterServicePlans(fake *FakeServicecatalogV1beta1) servicecatalogv1beta1.ClusterServicePlanInterface {
	return &fakeClusterServicePlans{
		gentype.NewFakeClientWithList[*v1beta1.ClusterServicePlan, *v1beta1.ClusterServicePlanList](
			fake.Fake,
			"",
			v1beta1.SchemeGroupVersion.WithResource("clusterserviceplans"),
			v1beta1.SchemeGroupVersion.WithKind("ClusterServicePlan"),
			func() *v1beta1.ClusterServicePlan { return &v1beta1.ClusterServicePlan{} },
			func() *v1beta1.ClusterServicePlanList { return &v1beta1.ClusterServicePlanList{} },
			func(dst, src *v1beta1.ClusterServicePlanList) { dst.ListMeta = src.ListMeta },
			func(list *v1beta1.ClusterServicePlanList) []*v1beta1.ClusterServicePlan {
				return gentype.ToPointerSlice(list.Items)
			},
			func(list *v1beta1.ClusterServicePlanList, items []*v1beta1.ClusterServicePlan) {
				list.Items = gentype.FromPointerSlice(items)
			},
		),
		fake,
	}
}
