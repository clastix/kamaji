# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
# VERSION ?= $(or $(shell git describe --abbrev=0 --tags 2>/dev/null),$(GIT_HEAD_COMMIT))

ifndef OVERRIDE_KAMAJI_VERSION
	VERSION = $(shell ./get-label.bash)
else
	ifneq ($(strip $(OVERRIDE_KAMAJI_VERSION)),)
		VERSION ?= $(OVERRIDE_KAMAJI_VERSION)
	else
		VERSION = $(shell ./get-label.bash)
	endif
endif

# ENVTEST_K8S_VERSION specifies the Kubernetes version to be used 
# during testing with the envtest environment. This ensures that 
# the tests run against the correct API and behavior for the 
# specific Kubernetes release being targeted (v1.31.0 in this case).
ENVTEST_K8S_VERSION = 1.31.0

# ENVTEST_VERSION defines the version of the setup-envtest binary 
# used to manage and download the Kubernetes binaries (like etcd, 
# kube-apiserver, and kubectl) required for testing. This version 
# ensures compatibility with the selected Kubernetes version and 
# must align closely with recent releases (release-0.19 is chosen here).
# Mismatches between these versions could result in compatibility issues.
ENVTEST_VERSION ?= release-0.19

# Image URL to use all building/pushing image targets
CONTAINER_REPOSITORY ?= quay.io/platform9/kamaji

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
KIND           ?= $(LOCALBIN)/kind
KO             ?= $(LOCALBIN)/ko
YQ             ?= $(LOCALBIN)/yq
ENVTEST        ?= $(LOCALBIN)/setup-envtest

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
ko: $(KO) ## Download ko locally if necessary.
$(KO): $(LOCALBIN)
	test -s $(LOCALBIN)/ko || GOBIN=$(LOCALBIN) go install github.com/google/ko@v0.14.1

.PHONY: yq
yq: $(YQ) ## Download yq locally if necessary.
$(YQ): $(LOCALBIN)
	test -s $(LOCALBIN)/yq || GOBIN=$(LOCALBIN) go install github.com/mikefarah/yq/v4@v4.44.2

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
	test -s $(LOCALBIN)/golangci-lint || GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2

.PHONY: apidocs-gen
apidocs-gen: $(APIDOCS_GEN)  ## Download crdoc locally if necessary.
$(APIDOCS_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/crdoc || GOBIN=$(LOCALBIN) go install fybrik.io/crdoc@latest

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)

##@ Development

rbac: controller-gen yq
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./..." output:stdout | $(YQ) '.rules' > ./charts/kamaji/controller-gen/clusterrole.yaml

webhook: controller-gen yq
	$(CONTROLLER_GEN) webhook paths="./..." output:stdout | $(YQ) 'select(documentIndex == 0) | .webhooks' > ./charts/kamaji/controller-gen/mutating-webhook.yaml
	$(CONTROLLER_GEN) webhook paths="./..." output:stdout | $(YQ) 'select(documentIndex == 1) | .webhooks' > ./charts/kamaji/controller-gen/validating-webhook.yaml
	$(YQ) -i 'map(.clientConfig.service.name |= "{{ include \"kamaji.webhookServiceName\" . }}")' ./charts/kamaji/controller-gen/mutating-webhook.yaml
	$(YQ) -i 'map(.clientConfig.service.namespace |= "{{ .Release.Namespace }}")' ./charts/kamaji/controller-gen/mutating-webhook.yaml
	$(YQ) -i 'map(.clientConfig.service.name |= "{{ include \"kamaji.webhookServiceName\" . }}")' ./charts/kamaji/controller-gen/validating-webhook.yaml
	$(YQ) -i 'map(.clientConfig.service.namespace |= "{{ .Release.Namespace }}")' ./charts/kamaji/controller-gen/validating-webhook.yaml

crds: controller-gen yq
	$(CONTROLLER_GEN) crd webhook paths="./..." output:stdout | $(YQ) 'select(documentIndex == 0)' > ./charts/kamaji/crds/kamaji.clastix.io_datastores.yaml
	$(CONTROLLER_GEN) crd webhook paths="./..." output:stdout | $(YQ) 'select(documentIndex == 1)' > ./charts/kamaji/crds/kamaji.clastix.io_tenantcontrolplanes.yaml
	$(YQ) -i '. *n load("./charts/kamaji/controller-gen/crd-conversion.yaml")' ./charts/kamaji/crds/kamaji.clastix.io_tenantcontrolplanes.yaml

manifests: rbac webhook crds ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

golint: golangci-lint ## Linting the code according to the styling guide.
	$(GOLANGCI_LINT) run -c .golangci.yml

## Run unit tests (all tests except E2E).
.PHONY: test
test: envtest ginkgo
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" $(GINKGO) -r -v --trace \
		./api/... \
		./cmd/... \
		./internal/... \
		-coverprofile cover.out

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

##@ Development

metallb:
	kubectl apply -f "https://raw.githubusercontent.com/metallb/metallb/$$(curl "https://api.github.com/repos/metallb/metallb/releases/latest" | jq -r ".tag_name")/config/manifests/metallb-native.yaml"
	kubectl wait pods -n metallb-system -l app=metallb,component=controller --for=condition=Ready --timeout=10m
	kubectl wait pods -n metallb-system -l app=metallb,component=speaker --for=condition=Ready --timeout=2m
	cat hack/metallb.yaml | sed -E "s|172.19|$$(docker network inspect -f '{{range .IPAM.Config}}{{.Gateway}}{{end}}' kind | sed -E 's|^([0-9]+\.[0-9]+)\..*$$|\1|g')|g" | kubectl apply -f -

cert-manager:
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) upgrade --install cert-manager jetstack/cert-manager --namespace certmanager-system --create-namespace --set "installCRDs=true"

load: kind
	$(KIND) load docker-image --name kamaji ${CONTAINER_REPOSITORY}:${VERSION}

##@ e2e

.PHONY: env
env:
	@make -C deploy/kind kind ingress-nginx

.PHONY: e2e
e2e: env build load helm ginkgo cert-manager ## Create a KinD cluster, install Kamaji on it and run the test suite.
	$(HELM) repo add clastix https://clastix.github.io/charts
	$(HELM) dependency build ./charts/kamaji
	$(HELM) upgrade --debug --install kamaji ./charts/kamaji --create-namespace --namespace kamaji-system --set "image.tag=$(VERSION)" --set "image.pullPolicy=Never" --set "telemetry.disabled=true"
	$(MAKE) datastores
	$(GINKGO) -v ./e2e

##@ Document

.PHONY: apidoc
apidoc: apidocs-gen
	$(APIDOCS_GEN) crdoc --resources charts/kamaji/crds --output docs/content/reference/api.md --template docs/templates/reference-cr.tmpl
