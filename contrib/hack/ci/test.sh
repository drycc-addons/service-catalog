#!/usr/bin/env bash
# Copyright 2017 The Kubernetes Authors.
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

# standard bash error handling
set -o nounset # treat unset variables as an error and exit immediately.
set -o errexit # exit immediately when a command fails.

source contrib/hack/ci/before_install.sh

if (( $DOCS_ONLY == 0 )); then
  echo "Running verify-docs"
  make verify-docs
else
  echo "Running full build"
  # make sure code quality is good and proper
  # generate the output binaries for server and client
  # ensure the tests build
  make verify build svcat test-unit build-integration
fi
