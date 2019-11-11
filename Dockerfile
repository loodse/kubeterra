# Copyright 2019 The KubeTerra Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


# Build the manager binary
FROM golang:1.13 as builder

ARG tag=dev
WORKDIR /workspace
COPY go.mod .
COPY go.sum .
COPY Makefile .
RUN make gomod
COPY . /workspace
RUN TAG=${tag} make build

# pack manager binary with UPX
FROM alpine as packer

ENV TERRAFORM_RELEASES_URL https://releases.hashicorp.com/terraform
ENV TERRAFORM_VERSION 0.12.7
ENV TERRAFORM_RELEASE ${TERRAFORM_RELEASES_URL}/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip
ENV TERRAFORM_RELEASE_CHECSUM ${TERRAFORM_RELEASES_URL}/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_SHA256SUMS

WORKDIR /workspace
COPY --from=builder /workspace/bin/kubeterra ./bin/
RUN apk add --no-cache upx coreutils
ADD $TERRAFORM_RELEASE .
ADD $TERRAFORM_RELEASE_CHECSUM .
RUN sha256sum --ignore-missing -c terraform_${TERRAFORM_VERSION}_SHA256SUMS
RUN unzip terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    mv terraform ./bin/
RUN upx ./bin/*

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:latest

WORKDIR /
RUN apk add --no-cache ca-certificates git
COPY --from=packer /workspace/bin/kubeterra .
COPY --from=packer /workspace/bin/terraform /usr/local/bin/
USER 65534:65534
CMD ["/kubeterra"]
