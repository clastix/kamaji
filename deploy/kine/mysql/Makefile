ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
NAME:=default
NAMESPACE:=mysql-system

mariadb: mariadb-certificates mariadb-secret mariadb-deployment

mariadb-certificates:
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

mariadb-secret:
	@kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@kubectl -n $(NAMESPACE) create secret generic mysql-$(NAME)-config \
		--from-file=$(ROOT_DIR)/certs/$(NAME)/ca.crt --from-file=$(ROOT_DIR)/certs/$(NAME)/ca.key \
		--from-file=$(ROOT_DIR)/certs/$(NAME)/server.key --from-file=$(ROOT_DIR)/certs/$(NAME)/server.crt \
		--from-file=$(ROOT_DIR)/mysql-ssl.cnf \
		--from-literal=MYSQL_ROOT_PASSWORD=root \
		--dry-run=client -o yaml | kubectl apply -f -

mariadb-deployment:
	@NAME=$(NAME) envsubst < $(ROOT_DIR)/mariadb.yaml | kubectl -n $(NAMESPACE) apply -f -

mariadb-destroy:
	@NAME=$(NAME) envsubst < $(ROOT_DIR)/mariadb.yaml | kubectl -n $(NAMESPACE) delete --ignore-not-found -f -
	@kubectl delete -n $(NAMESPACE) secret mysql-$(NAME)config --ignore-not-found
