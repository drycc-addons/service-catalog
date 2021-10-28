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

# ref: https://discourse.drone.io/t/how-to-get-the-commit-range-of-a-push-pr-event/1716/5
if [[ $DRONE_BUILD_EVENT == 'pull_request' ]]; then
  BASE=$(curl -s https://api.github.com/repos/$DRONE_REPO_OWNER/$DRONE_REPO_NAME/pulls/$DRONE_PULL_REQUEST | jq -r .base.sha)
elif [[ $DRONE_BUILD_EVENT == 'push' ]]; then
  BASE=$(curl -s https://api.github.com/repos/$DRONE_REPO_OWNER/$DRONE_REPO_NAME/commits/$DRONE_COMMIT_SHA | jq -r .parents[0].sha)
else
  echo "build event $DRONE_BUILD_EVENT not supported" >&2
  exit 1
fi
echo "$BASE..$DRONE_COMMIT_SHA"
