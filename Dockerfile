# Build the manager binary
FROM golang:1.12.7 as builder

WORKDIR /workspace
COPY go.mod .
COPY go.sum .
COPY Makefile .
RUN make gomod

COPY . /workspace

# Build
RUN make build

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:latest
WORKDIR /
COPY --from=builder /workspace/bin/kubeterra .
ENTRYPOINT ["/kubeterra"]
