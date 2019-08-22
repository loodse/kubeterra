# Build the manager binary
FROM golang:1.12.7 as builder

WORKDIR /workspace
COPY go.mod .
COPY go.sum .
COPY Makefile .
RUN make gomod
COPY . /workspace
RUN make build

# pack manager binary with UPX
FROM alpine as packer

ENV TERRAFORM_LINK https://releases.hashicorp.com/terraform/0.12.6/terraform_0.12.6_linux_amd64.zip
WORKDIR /workspace
RUN apk add --no-cache upx
ADD $TERRAFORM_LINK .
RUN unzip terraform_0.12.6_linux_amd64.zip && rm terraform_0.12.6_linux_amd64.zip
COPY --from=builder /workspace/bin/kubeterra .
RUN upx *

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:latest

WORKDIR /
RUN apk add --no-cache ca-certificates
COPY --from=packer /workspace/kubeterra .
COPY --from=packer /workspace/terraform /usr/local/bin/
CMD ["/kubeterra"]
