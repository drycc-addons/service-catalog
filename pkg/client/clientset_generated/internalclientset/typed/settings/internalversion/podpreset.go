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

package internalversion

import (
	context "context"

	settings "github.com/drycc-addons/service-catalog/pkg/apis/settings"
	scheme "github.com/drycc-addons/service-catalog/pkg/client/clientset_generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// PodPresetsGetter has a method to return a PodPresetInterface.
// A group's client should implement this interface.
type PodPresetsGetter interface {
	PodPresets(namespace string) PodPresetInterface
}

// PodPresetInterface has methods to work with PodPreset resources.
type PodPresetInterface interface {
	Create(ctx context.Context, podPreset *settings.PodPreset, opts v1.CreateOptions) (*settings.PodPreset, error)
	Update(ctx context.Context, podPreset *settings.PodPreset, opts v1.UpdateOptions) (*settings.PodPreset, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*settings.PodPreset, error)
	List(ctx context.Context, opts v1.ListOptions) (*settings.PodPresetList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *settings.PodPreset, err error)
	PodPresetExpansion
}

// podPresets implements PodPresetInterface
type podPresets struct {
	*gentype.ClientWithList[*settings.PodPreset, *settings.PodPresetList]
}

// newPodPresets returns a PodPresets
func newPodPresets(c *SettingsClient, namespace string) *podPresets {
	return &podPresets{
		gentype.NewClientWithList[*settings.PodPreset, *settings.PodPresetList](
			"podpresets",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *settings.PodPreset { return &settings.PodPreset{} },
			func() *settings.PodPresetList { return &settings.PodPresetList{} },
		),
	}
}
