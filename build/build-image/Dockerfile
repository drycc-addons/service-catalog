# Copyright 2016 The Kubernetes Authors.
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

ARG GO_VERSION
FROM docker.io/library/golang:${GO_VERSION}

RUN go install golang.org/x/lint/golint@latest

RUN apt-get update && \
    apt-get install jq etcd-server -y

# Create the full dir tree that we'll mount our src into when we run the image
RUN mkdir -p /go/src/github.com/kubernetes-sigs/service-catalog

# Default to our src dir
WORKDIR /go/src/github.com/kubernetes-sigs/service-catalog
