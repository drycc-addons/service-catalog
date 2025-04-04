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

// Code generated by informer-gen. DO NOT EDIT.

package v1beta1

import (
	context "context"
	time "time"

	apisservicecatalogv1beta1 "github.com/drycc-addons/service-catalog/pkg/apis/servicecatalog/v1beta1"
	clientset "github.com/drycc-addons/service-catalog/pkg/client/clientset_generated/clientset"
	internalinterfaces "github.com/drycc-addons/service-catalog/pkg/client/informers_generated/externalversions/internalinterfaces"
	servicecatalogv1beta1 "github.com/drycc-addons/service-catalog/pkg/client/listers_generated/servicecatalog/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ClusterServiceClassInformer provides access to a shared informer and lister for
// ClusterServiceClasses.
type ClusterServiceClassInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() servicecatalogv1beta1.ClusterServiceClassLister
}

type clusterServiceClassInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewClusterServiceClassInformer constructs a new informer for ClusterServiceClass type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewClusterServiceClassInformer(client clientset.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredClusterServiceClassInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredClusterServiceClassInformer constructs a new informer for ClusterServiceClass type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredClusterServiceClassInformer(client clientset.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ServicecatalogV1beta1().ClusterServiceClasses().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.ServicecatalogV1beta1().ClusterServiceClasses().Watch(context.TODO(), options)
			},
		},
		&apisservicecatalogv1beta1.ClusterServiceClass{},
		resyncPeriod,
		indexers,
	)
}

func (f *clusterServiceClassInformer) defaultInformer(client clientset.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredClusterServiceClassInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *clusterServiceClassInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&apisservicecatalogv1beta1.ClusterServiceClass{}, f.defaultInformer)
}

func (f *clusterServiceClassInformer) Lister() servicecatalogv1beta1.ClusterServiceClassLister {
	return servicecatalogv1beta1.NewClusterServiceClassLister(f.Informer().GetIndexer())
}
