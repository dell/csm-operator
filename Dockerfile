# Copyright © 2021 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build the manager binary
FROM golang:1.19 as builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY core/ core/
COPY pkg/ pkg/
COPY k8s/ k8s/
COPY tests/ tests/


# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

# Tag corresponding to digest 2db613439c1708842f3fd6104a6b7b831f1f9b1c223c225101649e6bc4ae2983 is 8.7-923
FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:2db613439c1708842f3fd6104a6b7b831f1f9b1c223c225101649e6bc4ae2983

RUN microdnf install yum \
    && yum -y update-minimal --security --sec-severity=Important --sec-severity=Critical \
    && yum clean all \
    && rpm -e yum \
    && microdnf clean all

ENV USER_UID=1001 \
    USER_NAME=dell-csm-operator
WORKDIR /
COPY --from=builder /workspace/manager .
COPY operatorconfig/ /etc/config/dell-csm-operator
LABEL vendor="Dell Inc." \
    name="dell-csm-operator" \
    summary="Operator for installing Dell CSI Drivers and Dell CSM Modules" \
    description="Common Operator for installing various Dell CSI Drivers and Dell CSM Modules" \
    version="1.0.0" \
    license="Dell CSM Operator Apache License"


ENTRYPOINT ["/manager"]
USER ${USER_UID}
