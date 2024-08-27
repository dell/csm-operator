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

ARG GOIMAGE

# base image, based on ubu-micro
ARG MICRODIR=/microdir
ARG PACKAGES_TO_INSTALL="rpm"
ARG UBIBASE
 # should be defined in docker.mk
ARG UBIBUILDER="registry.access.redhat.com/ubi9/ubi@sha256:1ee4d8c50d14d9c9e9229d9a039d793fcbc9aa803806d194c957a397cf1d2b17"

# --- microbase: image provided by RedHat as the UBI micro base image
FROM $UBIBASE AS microbase

# --- microbuilder: Used to add packages to the microbase image since no package manager exists in micro
FROM $UBIBUILDER AS microbuilder
ARG MICRODIR
ARG PACKAGES_TO_INSTALL
RUN mkdir ${MICRODIR}
COPY --from=microbase / ${MICRODIR}
RUN yum install \
  --installroot ${MICRODIR} \
  --releasever 9 \
  --setop install_weak_deps=false \
  -y \
  ${PACKAGES_TO_INSTALL}
RUN yum clean all \
  --installroot ${MICRODIR}

# --- baseimage: base image, build from microabse with packages installed by microbuilder
FROM $UBIBASE AS baseimage
ARG MICRODIR
COPY --from=microbuilder ${MICRODIR}/ .


# --- builder: builds the golang binary
FROM $GOIMAGE as builder

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

# --- final: resulting image, based upon the baseimage with the components compied from the builder
FROM baseimage as final
ENV USER_UID=1001 \
    USER_NAME=dell-csm-operator
WORKDIR /
COPY --from=builder /workspace/manager .
COPY operatorconfig/ /etc/config/dell-csm-operator
LABEL vendor="Dell Inc." \
    name="dell-csm-operator" \
    summary="Operator for installing Dell CSI Drivers and Dell CSM Modules" \
    description="Common Operator for installing various Dell CSI Drivers and Dell CSM Modules" \
    version="1.6.0" \
    license="Dell CSM Operator Apache License"

# copy the licenses folder
COPY licenses /licenses

ENTRYPOINT ["/manager"]
USER ${USER_UID}
