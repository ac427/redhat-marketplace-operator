# syntax = docker/dockerfile:experimental
FROM quay.io/rh-marketplace/golang-base:1.14 as builder
WORKDIR /usr/local/go/src/github.com/redhat-marketplace/redhat-marketplace-operator

ENV PATH=$PATH:/usr/local/go/bin CGO_ENABLED=0 GOOS=linux

COPY go.mod go.sum ./
COPY cmd cmd
COPY internal internal
COPY pkg pkg
COPY test test
COPY version version

RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
   go build -o build/_output/bin/redhat-marketplace-operator ./cmd/manager

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ARG app_version=latest

LABEL name="Red Hat Marketplace Operator" \
  maintainer="rhmoper@us.ibm.com" \
  vendor="Red Hat Marketplace" \
  release="1" \
  summary="Red Hat Marketplace Operator Image" \
  description="Operator for the Red Hat Marketplace" \
  version="${app_version}"

RUN microdnf update --setopt=tsflags=nodocs -y \
  && microdnf clean all \
  && rm -rf /var/cache/yum

ENV USER_UID=1001 \
  USER_NAME=redhat-marketplace-operator \
  ASSETS=/usr/local/bin/assets

# install operator binary
COPY --from=builder /usr/local/go/src/github.com/redhat-marketplace/redhat-marketplace-operator/build/_output/bin /usr/local/bin
COPY assets /usr/local/bin/assets
COPY build/bin/entrypoint /usr/local/bin/entrypoint
COPY build/bin/user_setup /usr/local/bin/user_setup
COPY LICENSE  /licenses/
RUN  /usr/local/bin/user_setup

WORKDIR /usr/local/bin
ENTRYPOINT ["/usr/local/bin/entrypoint"]
CMD ["redhat-marketplace-operator"]

USER ${USER_UID}
