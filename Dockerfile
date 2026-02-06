# Copyright © 2021-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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
ARG VERSION="1.11.1"

FROM $GOIMAGE as builder
ARG VERSION

RUN mkdir -p /go/src/csm-operator
COPY ./ /go/src/csm-operator

WORKDIR /go/src/csm-operator
RUN make build IMAGE_VERSION=$VERSION

FROM $BASEIMAGE as final
ARG VERSION
ENV USER_UID=1001 \
    USER_NAME=dell-csm-operator
WORKDIR /
COPY --from=builder /go/src/csm-operator/bin/manager .
COPY --from=builder /go/src/csm-operator/operatorconfig/ /etc/config/dell-csm-operator
LABEL vendor="Dell Technologies" \
    maintainer="Dell Technologies" \
    name="dell-csm-operator" \
    summary="Operator for installing Dell CSI Drivers and Dell CSM Modules" \
    description="Common Operator for installing various Dell CSI Drivers and Dell CSM Modules" \
    release="1.16.1" \
    version=$VERSION \
    license="Dell CSM Operator Apache License"

COPY licenses /licenses

ENTRYPOINT ["/manager"]
USER ${USER_UID}
