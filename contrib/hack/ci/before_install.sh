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

COMMIT_RANGE=`source contrib/hack/ci/commit_range.sh`
if [[ -z "$COMMIT_RANGE" ]]; then
    # Builds triggered by initial commit of a new branch.
    export DOCS_ONLY=0
else
    DOCS_ONLY=0
    DOCS_REGEX='(OWNERS|LICENSE)|(\.md$)|(^docs/)|(^docsite/)'
    if [[ -n "$(git diff --name-only $COMMIT_RANGE | grep -vE $DOCS_REGEX)" ]]; then DOCS_ONLY=1; fi
fi
if [[ $CI_COMMIT_TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+[a-z]*((-beta.[0-9]+)|(-(r|R)(c|C)[0-9]+))?$ ]]; then
    export DEPLOY_TYPE=release
elif [[ $CI_COMMIT_BRANCH == "main" ]]; then
    export DEPLOY_TYPE=main
else
    export DEPLOY_TYPE=none-$CI_COMMIT_TAG-$CI_COMMIT_BRANCH
fi
