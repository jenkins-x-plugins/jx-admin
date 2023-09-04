FROM ghcr.io/jenkins-x/jx-boot:latest

ARG BUILD_DATE
ARG VERSION
ARG REVISION
ARG TARGETARCH
ARG TARGETOS

LABEL org.opencontainers.image.source=https://github.com/jenkins-x-plugins/jx-admin
LABEL maintainer="jenkins-x"

# lets get the jx command to download the correct plugin version
ENV JX_ADMIN_VERSION $VERSION

RUN jx admin --help
