
# Image URL to use all building/pushing image targets (IMG will be overwritten during CI operations)
IMG ?= keikoproj/active-monitor:latest
LATEST_IMG ?= keikoproj/active-monitor:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Tools required to run the full suite of tests properly
OSNAME           ?= $(shell uname -s | tr A-Z a-z)
KUBEBUILDER_VER  ?= 3.0.0
KUBEBUILDER_ARCH ?= amd64

LOCAL  ?= true

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

export GO111MODULE = on

all: manager

mock:
	go install github.com/golang/mock/mockgen@v1.6.0
	@echo "mockgen is in progess"
	@for pkg in $(shell go list ./...) ; do \
		go generate ./... ;\
	done

# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: mock generate fmt vet manifests
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.7.2/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); LOCAL=$(LOCAL) go test ./api/... ./controllers/... ./metrics/... ./store/... -coverprofile coverage.txt

# Build manager binary
manager: mock generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crd/bases

# Uninstall CRDs from a cluster
uninstall:
	kubectl delete -f config/crd/bases

# Deploy controller in the Kubernetes cluster configured at ~/.kube/config
deploy: manifests
	kubectl apply -f config/crd/bases
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths=./api/...

# Build the docker image
docker-build: test
	docker build . -t ${IMG} -t ${LATEST_IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}
	docker push ${LATEST_IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1
CONTROLLER_GEN=$(shell go env GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

.PHONY: kubebuilder
kubebuilder:
	@echo "Downloading and installing Kubebuilder - this requires sudo privileges"
	curl -fsSL -O "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v$(KUBEBUILDER_VER)/kubebuilder_$(OSNAME)_$(KUBEBUILDER_ARCH)"
	chmod +x kubebuilder_$(OSNAME)_$(KUBEBUILDER_ARCH) && sudo mv kubebuilder_$(OSNAME)_$(KUBEBUILDER_ARCH) /usr/local/bin/
