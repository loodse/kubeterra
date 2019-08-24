# Build the manager binary
FROM golang:1.12.7 as builder

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
RUN apk add --no-cache ca-certificates
COPY --from=packer /workspace/bin/kubeterra .
COPY --from=packer /workspace/bin/terraform /usr/local/bin/
USER nobody:nobody
CMD ["/kubeterra"]
