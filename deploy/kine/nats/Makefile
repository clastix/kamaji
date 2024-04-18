ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
NAME:=default
NAMESPACE:=nats-system

.PHONY: helm
HELM = $(shell pwd)/../../../bin/helm
helm: ## Download helm locally if necessary.
	$(call go-install-tool,$(HELM),helm.sh/helm/v3/cmd/helm@v3.9.0)

nats: nats-certificates nats-secret nats-deployment

nats-certificates:
	rm -rf $(ROOT_DIR)/certs/$(NAME) && mkdir -p $(ROOT_DIR)/certs/$(NAME)
	cfssl gencert -initca $(ROOT_DIR)/ca-csr.json | cfssljson -bare $(ROOT_DIR)/certs/$(NAME)/ca
	@mv $(ROOT_DIR)/certs/$(NAME)/ca.pem $(ROOT_DIR)/certs/$(NAME)/ca.crt
	@mv $(ROOT_DIR)/certs/$(NAME)/ca-key.pem $(ROOT_DIR)/certs/$(NAME)/ca.key
	@NAME=$(NAME) NAMESPACE=$(NAMESPACE) envsubst < server-csr.json > $(ROOT_DIR)/certs/$(NAME)/server-csr.json
	cfssl gencert -ca=$(ROOT_DIR)/certs/$(NAME)/ca.crt -ca-key=$(ROOT_DIR)/certs/$(NAME)/ca.key \
		-config=$(ROOT_DIR)/config.json -profile=server \
		$(ROOT_DIR)/certs/$(NAME)/server-csr.json | cfssljson -bare $(ROOT_DIR)/certs/$(NAME)/server
	@mv $(ROOT_DIR)/certs/$(NAME)/server.pem $(ROOT_DIR)/certs/$(NAME)/server.crt
	@mv $(ROOT_DIR)/certs/$(NAME)/server-key.pem $(ROOT_DIR)/certs/$(NAME)/server.key
	chmod 644 $(ROOT_DIR)/certs/$(NAME)/*

nats-secret:
	@kubectl create namespace $(NAMESPACE) || true
	@kubectl -n $(NAMESPACE) create secret generic nats-$(NAME)-config \
		--from-file=$(ROOT_DIR)/certs/$(NAME)/ca.crt --from-file=$(ROOT_DIR)/certs/$(NAME)/ca.key \
		--from-file=$(ROOT_DIR)/certs/$(NAME)/server.key --from-file=$(ROOT_DIR)/certs/$(NAME)/server.crt \
		--dry-run=client -o yaml | kubectl apply -f -

nats-deployment:
	@NAME=$(NAME) envsubst < $(ROOT_DIR)/values.yaml | $(HELM) upgrade --install $(NAME) nats/nats --create-namespace -n nats-system -f -

nats-destroy:
	@NAME=$(NAME) envsubst < $(ROOT_DIR)/nats.yaml | kubectl -n $(NAMESPACE) delete --ignore-not-found -f -
	@kubectl delete -n $(NAMESPACE) secret mysql-$(NAME)config --ignore-not-found
