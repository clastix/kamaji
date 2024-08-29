# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= v1.0.0

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# clastix.io/operator-bundle:$VERSION and clastix.io/operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= clastix.io/operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:$(VERSION)

# Image URL to use all building/pushing image targets
CONTAINER_REPOSITORY ?= docker.io/clastix/kamaji

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
APIDOCS_GEN    ?= $(LOCALBIN)/crdoc
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
GINKGO         ?= $(LOCALBIN)/ginkgo
GOLANGCI_LINT  ?= $(LOCALBIN)/golangci-lint
HELM           ?= $(LOCALBIN)/helm
KUSTOMIZE      ?= $(LOCALBIN)/kustomize
KIND           ?= $(LOCALBIN)/kind
KO             ?= $(LOCALBIN)/ko

all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Binary

.PHONY: ko
ko: $(HELM) ## Download ko locally if necessary.
$(KO): $(LOCALBIN)
	test -s $(LOCALBIN)/ko || GOBIN=$(LOCALBIN) go install github.com/google/ko@v0.14.1

.PHONY: helm
helm: $(HELM) ## Download helm locally if necessary.
$(HELM): $(LOCALBIN)
	test -s $(LOCALBIN)/helm || GOBIN=$(LOCALBIN) go install helm.sh/helm/v3/cmd/helm@v3.9.0

.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary.
$(GINKGO): $(LOCALBIN)
	test -s $(LOCALBIN)/ginkgo || GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	test -s $(LOCALBIN)/kind || GOBIN=$(LOCALBIN) go install sigs.k8s.io/kind/cmd/kind@v0.14.0

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.1

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	test -s $(LOCALBIN)/golangci-lint || GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2

.PHONY: apidocs-gen
apidocs-gen: $(APIDOCS_GEN)  ## Download crdoc locally if necessary.
$(APIDOCS_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/crdoc || GOBIN=$(LOCALBIN) go install fybrik.io/crdoc@latest

kustomize: ## Download kustomize locally if necessary.
	$(call install-kustomize,$(KUSTOMIZE),3.8.7)

##@ Development

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

golint: golangci-lint ## Linting the code according to the styling guide.
	$(GOLANGCI_LINT) run -c .golangci.yml

test:
	go test ./... -coverprofile cover.out

_datastore-mysql:
	$(MAKE) NAME=$(NAME) -C deploy/kine/mysql mariadb
	kubectl apply -f $(shell pwd)/config/samples/kamaji_v1alpha1_datastore_mysql_$(NAME).yaml

datastore-mysql:
	$(MAKE) NAME=bronze _datastore-mysql
	$(MAKE) NAME=silver _datastore-mysql
	$(MAKE) NAME=gold _datastore-mysql

_datastore-postgres:
	$(MAKE) NAME=$(NAME) NAMESPACE=postgres-system -C deploy/kine/postgresql postgresql
	kubectl apply -f $(shell pwd)/config/samples/kamaji_v1alpha1_datastore_postgresql_$(NAME).yaml

datastore-postgres:
	$(MAKE) NAME=bronze _datastore-postgres
	$(MAKE) NAME=silver _datastore-postgres
	$(MAKE) NAME=gold _datastore-postgres

_datastore-etcd:
	$(HELM) upgrade --install etcd-$(NAME) clastix/kamaji-etcd --create-namespace -n etcd-system --set datastore.enabled=true --set fullnameOverride=etcd-$(NAME)

_datastore-nats:
	$(MAKE) NAME=$(NAME) NAMESPACE=nats-system -C deploy/kine/nats nats
	kubectl apply -f $(shell pwd)/config/samples/kamaji_v1alpha1_datastore_nats_$(NAME).yaml

datastore-etcd: helm
	$(HELM) repo add clastix https://clastix.github.io/charts
	$(HELM) repo update
	$(MAKE) NAME=bronze _datastore-etcd
	$(MAKE) NAME=silver _datastore-etcd
	$(MAKE) NAME=gold _datastore-etcd

datastore-nats: helm
	$(HELM) repo add nats https://nats-io.github.io/k8s/helm/charts/
	$(HELM) repo update
	$(MAKE) NAME=bronze _datastore-nats
	$(MAKE) NAME=silver _datastore-nats
	$(MAKE) NAME=gold _datastore-nats
	$(MAKE) NAME=notls _datastore-nats

datastores: datastore-mysql datastore-etcd datastore-postgres datastore-nats ## Install all Kamaji DataStores with multiple drivers, and different tiers.

##@ Build

# Get information about git current status
GIT_HEAD_COMMIT ?= $$(git rev-parse --short HEAD)
GIT_TAG_COMMIT  ?= $$(git rev-parse --short $(VERSION))
GIT_MODIFIED_1  ?= $$(git diff $(GIT_HEAD_COMMIT) $(GIT_TAG_COMMIT) --quiet && echo "" || echo ".dev")
GIT_MODIFIED_2  ?= $$(git diff --quiet && echo "" || echo ".dirty")
GIT_MODIFIED    ?= $$(echo "$(GIT_MODIFIED_1)$(GIT_MODIFIED_2)")
GIT_REPO        ?= $$(git config --get remote.origin.url)
BUILD_DATE      ?= $$(git log -1 --format="%at" | xargs -I{} date -d @{} +%Y-%m-%dT%H:%M:%S)

LD_FLAGS ?= "-X github.com/clastix/kamaji/internal.GitCommit=$(GIT_HEAD_COMMIT) \
             -X github.com/clastix/kamaji/internal.GitTag=$(VERSION) \
             -X github.com/clastix/kamaji/internal.GitDirty=$(GIT_MODIFIED) \
             -X github.com/clastix/kamaji/internal.BuildTime=$(BUILD_DATE) \
             -X github.com/clastix/kamaji/internal.GitRepo=$(GIT_REPO)"

KO_PUSH ?= false
KO_LOCAL ?= true

run: manifests generate ## Run a controller from your host.
	go run ./main.go

build: $(KO)
	LD_FLAGS=$(LD_FLAGS) \
	KOCACHE=/tmp/ko-cache KO_DOCKER_REPO=${CONTAINER_REPOSITORY} \
	$(KO) build ./ --bare --tags=$(VERSION) --local=$(KO_LOCAL) --push=$(KO_PUSH)

##@ Deployment

metallb:
	kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.7/config/manifests/metallb-native.yaml
	kubectl apply -f https://kind.sigs.k8s.io/examples/loadbalancer/metallb-config.yaml
	echo ""
	docker network inspect -f '{{.IPAM.Config}}' kind

cert-manager:
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.10.1/cert-manager.yaml

dev: generate manifests uninstall install rbac ## Full installation for development purposes
	go fmt ./...

load: kind build
	$(KIND) load docker-image --name kamaji ${CONTAINER_REPOSITORY}:${VERSION}

rbac: manifests kustomize ## Install RBAC into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/rbac | kubectl apply -f -

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${CONTAINER_REPOSITORY}:${VERSION}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

yaml-installation-file: manifests kustomize ## Create yaml installation file
	$(KUSTOMIZE) build config/default > config/install.yaml

.PHONY: bundle
bundle: manifests kustomize ## Generate bundle manifests and metadata, then validate generated files.
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.15.1/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)

define install-kustomize
@[ -f $(1) ] || { \
set -e ;\
echo "Installing v$(2)" ;\
cd bin ;\
wget "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh" ;\
bash ./install_kustomize.sh $(2) ;\
}
endef

.PHONY: env
env:
	@make -C deploy/kind kind ingress-nginx

##@ e2e

.PHONY: e2e
e2e: env load helm ginkgo cert-manager ## Create a KinD cluster, install Kamaji on it and run the test suite.
	$(HELM) repo add clastix https://clastix.github.io/charts
	$(HELM) dependency build ./charts/kamaji
	$(HELM) upgrade --debug --install kamaji ./charts/kamaji --create-namespace --namespace kamaji-system --set "image.pullPolicy=Never" --set "telemetry.disabled=true"
	$(MAKE) datastores
	$(GINKGO) -v ./e2e

##@ Document

.PHONY: apidoc
apidoc: apidocs-gen
	$(APIDOCS_GEN) crdoc --resources config/crd/bases --output docs/content/reference/api.md --template docs/templates/reference-cr.tmpl
