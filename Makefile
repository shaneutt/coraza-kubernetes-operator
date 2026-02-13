# ------------------------------------------------------------------------------
# Vars
# ------------------------------------------------------------------------------

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

CONTAINER_TOOL ?= docker

KIND_CLUSTER_NAME ?= coraza-kubernetes-operator-integration
ISTIO_VERSION ?= 1.28.2

VERSION ?= dev
CONTROLLER_MANAGER_CONTAINER_IMAGE_BASE ?= ghcr.io/networking-incubator/coraza-kubernetes-operator
CONTROLLER_MANAGER_CONTAINER_IMAGE_TAG ?= $(VERSION)
CONTROLLER_MANAGER_CONTAINER_IMAGE ?= ${CONTROLLER_MANAGER_CONTAINER_IMAGE_BASE}:${CONTROLLER_MANAGER_CONTAINER_IMAGE_TAG}

# ------------------------------------------------------------------------------
# General
# ------------------------------------------------------------------------------

.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: all
all: build

# ------------------------------------------------------------------------------
# Build
# ------------------------------------------------------------------------------

.PHONY: build
build: manifests generate fmt vet lint
	go build -o bin/manager cmd/main.go

.PHONY: build.image
build.image:
	$(CONTAINER_TOOL) build -t ${CONTROLLER_MANAGER_CONTAINER_IMAGE} .

.PHONY: build.installer
build.installer: manifests generate kustomize
	mkdir -p dist
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${CONTROLLER_MANAGER_CONTAINER_IMAGE}
	"$(KUSTOMIZE)" build config/default > dist/install.yaml

.PHONY: release.manifests
release.manifests: manifests generate kustomize
	@echo "Building release manifest bundles..."
	@mkdir -p dist
	@echo "Building CRDs bundle..."
	"$(KUSTOMIZE)" build config/crd > dist/crds.yaml
	@echo "Building controller-manager bundle..."
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${CONTROLLER_MANAGER_CONTAINER_IMAGE_BASE}:${VERSION}
	"$(KUSTOMIZE)" build config/manager > dist/operator.yaml
	@echo "Building samples bundle..."
	cat config/samples/gateway.yaml > dist/samples.yaml
	echo "---" >> dist/samples.yaml
	cat config/samples/ruleset.yaml >> dist/samples.yaml
	echo "---" >> dist/samples.yaml
	cat config/samples/engine.yaml >> dist/samples.yaml
	@echo "Manifest bundles built successfully in dist/"
	@ls -lh dist/

# ------------------------------------------------------------------------------
# Deployment
# ------------------------------------------------------------------------------

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply -f -; else echo "No CRDs to install; skipping."; fi

.PHONY: uninstall
uninstall: manifests kustomize
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -; else echo "No CRDs to delete; skipping."; fi

.PHONY: deploy
deploy: manifests kustomize
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${CONTROLLER_MANAGER_CONTAINER_IMAGE}
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" apply -f -

.PHONY: undeploy
undeploy: kustomize
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: run
run: manifests generate fmt vet
	go run ./cmd/main.go

# ------------------------------------------------------------------------------
# Development
# ------------------------------------------------------------------------------

.PHONY: manifests
manifests: controller-gen
	"$(CONTROLLER_GEN)" rbac:roleName=coraza-controller-manager crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint: golangci-lint
	"$(GOLANGCI_LINT)" run --build-tags integration ./...

.PHONY: lint.fix
lint.fix: golangci-lint
	"$(GOLANGCI_LINT)" run --fix --build-tags integration ./...

.PHONY: lint.config
lint.config: golangci-lint
	"$(GOLANGCI_LINT)" config verify

# ------------------------------------------------------------------------------
# Test Cluster
# ------------------------------------------------------------------------------

.PHONY: cluster.kind
cluster.kind:
	ISTIO_VERSION=${ISTIO_VERSION} python3 hack/kind_cluster.py setup

.PHONY: cluster.load-images
cluster.load-images:
	@$(CONTAINER_TOOL) exec ${KIND_CLUSTER_NAME}-control-plane crictl rmi ${CONTROLLER_MANAGER_CONTAINER_IMAGE} 2>/dev/null || true
	$(KIND) load docker-image ${CONTROLLER_MANAGER_CONTAINER_IMAGE} --name ${KIND_CLUSTER_NAME}

.PHONY: clean.cluster.kind
clean.cluster.kind:
	python3 hack/kind_cluster.py delete --name ${KIND_CLUSTER_NAME}

# -------------------------------------------------------------------------------
# Testing
# -------------------------------------------------------------------------------

.PHONY: test
test: generate
	ISTIO_VERSION=${ISTIO_VERSION} go test -v ./...

.PHONY: test.coverage
test.coverage: generate
	@echo "Running tests with coverage..."
	@ISTIO_VERSION=${ISTIO_VERSION} go test -v ./... -coverprofile=coverage.out -covermode=atomic
	@echo "Coverage by package:"
	@go tool cover -func=coverage.out | grep -v "total:" || true
	@echo "Total coverage:"
	@total=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Total: $${total}%"

.PHONY: test.integration
test.integration:
	go clean -testcache
	KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME} ISTIO_VERSION=${ISTIO_VERSION} go test -tags=integration ./test/integration/... -v

# -------------------------------------------------------------------------------
# Dependencies
# -------------------------------------------------------------------------------

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

KUBECTL ?= kubectl
KIND ?= kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

KUSTOMIZE_VERSION ?= v5.7.1
CONTROLLER_TOOLS_VERSION ?= v0.19.0
GOLANGCI_LINT_VERSION ?= v2.5.0

.PHONY: kustomize
kustomize: $(KUSTOMIZE)
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN)
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT)
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef
