ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
NAME:=default
NAMESPACE:=kamaji-system

postgresql: cnpg-setup cnpg-deploy postgresql-secret

cnpg-setup:
	@kubectl apply -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/main/releases/cnpg-1.18.0.yaml

cnpg-deploy:
	@kubectl -n cnpg-system rollout status deployment/cnpg-controller-manager
	@kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@NAME=$(NAME) envsubst < postgresql.yaml | kubectl -n $(NAMESPACE) apply -f -
	@while ! kubectl -n $(NAMESPACE) get secret postgres-$(NAME)-superuser > /dev/null 2>&1; do sleep 1; done

CNPG = $(shell git rev-parse --show-toplevel)/bin/kubectl-cnpg
cnpg:
	@test -f $(shell git rev-parse --show-toplevel)/bin/kubectl-cnpg || curl -sSfL \
      https://github.com/cloudnative-pg/cloudnative-pg/raw/main/hack/install-cnpg-plugin.sh | \
      sh -s -- -b $(shell git rev-parse --show-toplevel)/bin

postgresql-secret: cnpg
	@kubectl -n $(NAMESPACE) get secret postgres-$(NAME)-root-cert > /dev/null 2>&1 || $(CNPG) -n $(NAMESPACE) certificate postgres-$(NAME)-root-cert \
    		--cnpg-cluster postgres-$(NAME) \
    		--cnpg-user $$(kubectl -n $(NAMESPACE) get secret postgres-$(NAME)-superuser -o jsonpath='{.data.username}' | base64 -d)

postgresql-destroy:
	@NAME=$(NAME) envsubst < postgresql.yaml | kubectl -n $(NAMESPACE) delete -f -
	@kubectl -n $(NAMESPACE) delete secret postgres-$(NAME)-root-cert --ignore-not-found
