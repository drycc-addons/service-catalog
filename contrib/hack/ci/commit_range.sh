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

# ref: https://woodpecker-ci.org/docs/next/usage/environment
if [[ $CI_PIPELINE_EVENT == 'pull_request' ]]; then
  BASE=$(curl -s https://api.github.com/repos/$CI_REPO_OWNER/$CI_REPO_NAME/pulls/$CI_COMMIT_PULL_REQUEST | jq -r .base.sha)
elif [[ "push tag" =~  "${CI_PIPELINE_EVENT}" ]]; then
  BASE=$(curl -s https://api.github.com/repos/$CI_REPO_OWNER/$CI_REPO_NAME/commits/$CI_COMMIT_SHA | jq -r .parents[0].sha)
else
  echo "build event $CI_PIPELINE_EVENT not supported" >&2
  exit 1
fi
echo "$BASE..$CI_COMMIT_SHA"
