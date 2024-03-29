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

// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/drycc-addons/service-catalog/pkg/apis/settings
// +k8s:openapi-gen=true
// +k8s:defaulter-gen=TypeMeta

// Package v1alpha1 defines all of the versioned (v1alpha1) definitions
// of the settings group.
// +groupName=settings.servicecatalog.k8s.io
package v1alpha1 // import "github.com/drycc-addons/service-catalog/pkg/apis/settings/v1alpha1"
