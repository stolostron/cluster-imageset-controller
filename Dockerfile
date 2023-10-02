FROM registry.ci.openshift.org/stolostron/builder:go1.20-linux AS builder
WORKDIR /go/src/github.com/stolostron/cluster-imageset-controller
COPY . .
ENV GO_PACKAGE github.com/stolostron/cluster-imageset-controller

# Build
RUN make build --warn-undefined-variables

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

# Add the binaries
COPY --from=builder /go/src/github.com/stolostron/cluster-imageset-controller/bin/clusterimageset .
