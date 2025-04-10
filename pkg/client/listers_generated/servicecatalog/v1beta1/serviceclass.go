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

// ServiceClassLister helps list ServiceClasses.
// All objects returned here must be treated as read-only.
type ServiceClassLister interface {
	// List lists all ServiceClasses in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*servicecatalogv1beta1.ServiceClass, err error)
	// ServiceClasses returns an object that can list and get ServiceClasses.
	ServiceClasses(namespace string) ServiceClassNamespaceLister
	ServiceClassListerExpansion
}

// serviceClassLister implements the ServiceClassLister interface.
type serviceClassLister struct {
	listers.ResourceIndexer[*servicecatalogv1beta1.ServiceClass]
}

// NewServiceClassLister returns a new ServiceClassLister.
func NewServiceClassLister(indexer cache.Indexer) ServiceClassLister {
	return &serviceClassLister{listers.New[*servicecatalogv1beta1.ServiceClass](indexer, servicecatalogv1beta1.Resource("serviceclass"))}
}

// ServiceClasses returns an object that can list and get ServiceClasses.
func (s *serviceClassLister) ServiceClasses(namespace string) ServiceClassNamespaceLister {
	return serviceClassNamespaceLister{listers.NewNamespaced[*servicecatalogv1beta1.ServiceClass](s.ResourceIndexer, namespace)}
}

// ServiceClassNamespaceLister helps list and get ServiceClasses.
// All objects returned here must be treated as read-only.
type ServiceClassNamespaceLister interface {
	// List lists all ServiceClasses in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*servicecatalogv1beta1.ServiceClass, err error)
	// Get retrieves the ServiceClass from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*servicecatalogv1beta1.ServiceClass, error)
	ServiceClassNamespaceListerExpansion
}

// serviceClassNamespaceLister implements the ServiceClassNamespaceLister
// interface.
type serviceClassNamespaceLister struct {
	listers.ResourceIndexer[*servicecatalogv1beta1.ServiceClass]
}
