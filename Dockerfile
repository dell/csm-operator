# Build the manager binary
FROM golang:1.18 as builder

WORKDIR /workspace

# Download repctl
RUN mkdir /repctl
RUN wget  https://github.com/dell/csm-replication/releases/download/v1.2.0/repctl-linux-amd64 -O /repctl/repctl
RUN cd /repctl && chmod +x repctl && mv repctl /usr/local/bin

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

FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:21504085e8d740e62b52573fe9a1a0d58a3e7dba589cac69734ad2fa81d66635

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
    version="0.2.0" \
    license="Dell CSM Operator Apache License"

ENTRYPOINT ["/manager"]
USER ${USER_UID}

