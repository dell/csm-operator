# Copyright © 2021 - 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
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

ARG BASEIMAGE
ARG GOIMAGE

FROM $GOIMAGE as builder
WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY core/ core/
COPY pkg/ pkg/
COPY k8s/ k8s/
COPY vendor/ vendor/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod=vendor -a -o manager main.go

FROM $BASEIMAGE as final
ENV USER_UID=1001 \
    USER_NAME=dell-csm-operator
WORKDIR /
COPY --from=builder /workspace/manager .
COPY operatorconfig/ /etc/config/dell-csm-operator
LABEL vendor="Dell Technologies" \
    maintainer="Dell Technologies" \
    name="dell-csm-operator" \
    summary="Operator for installing Dell CSI Drivers and Dell CSM Modules" \
    description="Common Operator for installing various Dell CSI Drivers and Dell CSM Modules" \
    release="1.16.0" \
    version="1.11.0" \
    license="Dell CSM Operator Apache License"

COPY licenses /licenses

ENTRYPOINT ["/manager"]
USER ${USER_UID}
