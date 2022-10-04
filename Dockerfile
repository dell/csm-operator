# Build the manager binary
FROM golang:1.19.1 as builder

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

# Tag corresponding to digest d1f8eff6032334a81d7cbfd73dacee680e8138db57ecbc91548b97bb45e698e5 is 8.6-902.1661794353
FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:c8c1c0f893a7ba679fd65863f2d1389179a92957c31e95521b3290c6b6fc4a76

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
    version="0.3.0" \
    license="Dell CSM Operator Apache License"


ENTRYPOINT ["/manager"]
USER ${USER_UID}
