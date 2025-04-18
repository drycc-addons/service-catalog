#!/bin/bash

# Copyright 2015 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# The contents of this file are in a specific order
# Listers depend on the base client
# Informers depend on listers

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

REPO_ROOT=$(realpath $(dirname "${BASH_SOURCE}")/..)
BINDIR=${REPO_ROOT}/bin

# Generate the internal clientset (pkg/client/clientset_generated/internalclientset)
${BINDIR}/client-gen "$@" \
	      --input-base "github.com/drycc-addons/service-catalog/pkg/apis/" \
	      --input settings/ \
	      --clientset-path "github.com/drycc-addons/service-catalog/pkg/client/clientset_generated/" \
	      --clientset-name internalclientset \
	      --go-header-file "contrib/hack/boilerplate.go.txt" \
		  --output-dir pkg/client/clientset_generated
# Generate the versioned clientset (pkg/client/clientset_generated/clientset)
${BINDIR}/client-gen "$@" \
        --input-base "github.com/drycc-addons/service-catalog/pkg/apis/" \
	      --input "servicecatalog/v1beta1" \
	      --input "settings/v1alpha1" \
	      --clientset-path "github.com/drycc-addons/service-catalog/pkg/client/clientset_generated/" \
	      --clientset-name "clientset" \
	      --go-header-file "contrib/hack/boilerplate.go.txt" \
		  --output-dir pkg/client/clientset_generated
# generate listers after having the base client generated, and before informers
${BINDIR}/lister-gen "$@" \
	      --output-pkg "github.com/drycc-addons/service-catalog/pkg/client/listers_generated" \
		  --output-dir "pkg/client/listers_generated" \
	      --go-header-file "contrib/hack/boilerplate.go.txt" \
	      "github.com/drycc-addons/service-catalog/pkg/apis/servicecatalog/v1beta1" \
	      "github.com/drycc-addons/service-catalog/pkg/apis/settings" \
	      "github.com/drycc-addons/service-catalog/pkg/apis/settings/v1alpha1"
# generate informers after the listers have been generated
${BINDIR}/informer-gen "$@" \
	      --go-header-file "contrib/hack/boilerplate.go.txt" \
	      --internal-clientset-package "github.com/drycc-addons/service-catalog/pkg/client/clientset_generated/internalclientset" \
	      --versioned-clientset-package "github.com/drycc-addons/service-catalog/pkg/client/clientset_generated/clientset" \
	      --listers-package "github.com/drycc-addons/service-catalog/pkg/client/listers_generated" \
	      --output-pkg "github.com/drycc-addons/service-catalog/pkg/client/informers_generated" \
		  --output-dir "pkg/client/informers_generated" \
	      "github.com/drycc-addons/service-catalog/pkg/apis/servicecatalog/v1beta1" \
	      "github.com/drycc-addons/service-catalog/pkg/apis/settings" \
	      "github.com/drycc-addons/service-catalog/pkg/apis/settings/v1alpha1"
