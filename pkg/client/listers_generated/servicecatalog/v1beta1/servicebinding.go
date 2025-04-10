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

// Code generated by lister-gen. DO NOT EDIT.

package v1beta1

import (
	servicecatalogv1beta1 "github.com/drycc-addons/service-catalog/pkg/apis/servicecatalog/v1beta1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// ServiceBindingLister helps list ServiceBindings.
// All objects returned here must be treated as read-only.
type ServiceBindingLister interface {
	// List lists all ServiceBindings in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*servicecatalogv1beta1.ServiceBinding, err error)
	// ServiceBindings returns an object that can list and get ServiceBindings.
	ServiceBindings(namespace string) ServiceBindingNamespaceLister
	ServiceBindingListerExpansion
}

// serviceBindingLister implements the ServiceBindingLister interface.
type serviceBindingLister struct {
	listers.ResourceIndexer[*servicecatalogv1beta1.ServiceBinding]
}

// NewServiceBindingLister returns a new ServiceBindingLister.
func NewServiceBindingLister(indexer cache.Indexer) ServiceBindingLister {
	return &serviceBindingLister{listers.New[*servicecatalogv1beta1.ServiceBinding](indexer, servicecatalogv1beta1.Resource("servicebinding"))}
}

// ServiceBindings returns an object that can list and get ServiceBindings.
func (s *serviceBindingLister) ServiceBindings(namespace string) ServiceBindingNamespaceLister {
	return serviceBindingNamespaceLister{listers.NewNamespaced[*servicecatalogv1beta1.ServiceBinding](s.ResourceIndexer, namespace)}
}

// ServiceBindingNamespaceLister helps list and get ServiceBindings.
// All objects returned here must be treated as read-only.
type ServiceBindingNamespaceLister interface {
	// List lists all ServiceBindings in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*servicecatalogv1beta1.ServiceBinding, err error)
	// Get retrieves the ServiceBinding from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*servicecatalogv1beta1.ServiceBinding, error)
	ServiceBindingNamespaceListerExpansion
}

// serviceBindingNamespaceLister implements the ServiceBindingNamespaceLister
// interface.
type serviceBindingNamespaceLister struct {
	listers.ResourceIndexer[*servicecatalogv1beta1.ServiceBinding]
}
