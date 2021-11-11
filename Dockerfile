# Build the manager binary
FROM golang:1.16 as builder

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

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

FROM gcr.io/distroless/static:nonroot

ENV USER_UID=1001 \
    USER_NAME=dell-csi-operator \
    X_CSM_OPERATOR_CONFIG_DIR="/etc/config/csm-operator"
WORKDIR /
COPY --from=builder /workspace/manager .
COPY operatorconfig/ /etc/config/local/csm-operator
LABEL vendor="Dell Inc." \
    name="csm-operator" \
    summary="Operator for installing Dell EMC CSM Modules" \
    description="Common Operator for installing various Dell EMC Container Storage Modules" \
    version="1.5.0" \
    license="Dell CSI Operator Apache License"

ENTRYPOINT ["/manager"]
USER ${USER_UID}

