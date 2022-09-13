ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

postgresql: cnpg-setup cnpg-deploy postgresql-secret

cnpg-setup:
	@kubectl apply -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/main/releases/cnpg-1.16.0.yaml

cnpg-deploy:
	@kubectl -n cnpg-system rollout status deployment/cnpg-controller-manager
	@kubectl -n kamaji-system apply -f postgresql.yaml
	@while ! kubectl -n kamaji-system get secret postgresql-superuser > /dev/null 2>&1; do sleep 1; done

CNPG = $(shell git rev-parse --show-toplevel)/bin/kubectl-cnpg
cnpg:
	@test -f $(shell git rev-parse --show-toplevel)/bin/kubectl-cnpg || curl -sSfL \
      https://github.com/cloudnative-pg/cloudnative-pg/raw/main/hack/install-cnpg-plugin.sh | \
      sh -s -- -b $(shell git rev-parse --show-toplevel)/bin

postgresql-secret: cnpg
	@kubectl -n kamaji-system get secret postgres-root-cert > /dev/null 2>&1 || $(CNPG) -n kamaji-system certificate postgres-root-cert \
    		--cnpg-cluster postgresql \
    		--cnpg-user $$(kubectl -n kamaji-system get secret postgresql-superuser -o jsonpath='{.data.username}' | base64 -d)

postgresql-destroy:
	@kubectl delete -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/main/releases/cnpg-1.16.0.yaml --ignore-not-found && \
	kubectl -n kamaji-system delete secret postgres-root-cert --ignore-not-found
