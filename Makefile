.DEFAULT_GOAL:=help

export CGO_ENABLED=0
export GOPROXY=https://proxy.golang.org
export GO111MODULE=on
export GOFLAGS?=-mod=readonly

CONTROLLER_GEN=go run sigs.k8s.io/controller-tools/cmd/controller-gen
CRD_OPTIONS ?= "crd:trivialVersions=true"
REGISTRY ?= docker.io/kubermatic
CONTROLLER_IMG ?= $(REGISTRY)/kubeterra
TAG ?= dev
GO_LDFLAGS = -s -w -X github.com/loodse/kubeterra/resources.Image=$(CONTROLLER_IMG):$(TAG)

test: generate manifests ## Run tests
	go test ./api/... ./controllers/... -coverprofile cover.out

manager: generate build ## Generate code, build manager binary

build: ## Build manager binary
	go build -ldflags '$(GO_LDFLAGS)' -v -o bin/kubeterra .

pack:
	upx ./bin/*

run: generate ## Run against the configured Kubernetes cluster in ~/.kube/config
	go run *.go manager -d

install: manifests ## Install CRDs into a cluster
	kubectl apply -f config/crd/bases

deploy: manifests ## Deploy controller in the configured Kubernetes cluster in ~/.kube/config
	kubectl apply -f config/crd/bases
	kustomize build config/default | kubectl apply -f -

manifests: ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=kubeterra webhook paths="./..." output:crd:artifacts:config=config/crd/bases

lint: ## Run golangci-lint against code
	golangci-lint run

gen: generate manifests ## Shortcut to generate code and manifests

generate: ## Generate code
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths=./api/...

docker-build: ## Build the docker image
	docker build . -t ${CONTROLLER_IMG}:${TAG} --pull --build-arg tag=$(TAG)
	@echo "updating kustomize image patch file for manager resource"
	sed -i '' -e 's@image: .*@image: '"${CONTROLLER_IMG}:${TAG}"'@' ./config/default/manager_image_patch.yaml

gomod:
	go mod download

kind:
	-kind delete cluster
	kind create cluster

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
