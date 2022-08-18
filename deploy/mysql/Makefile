ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

mariadb: mariadb-certificates mariadb-secret mariadb-kine-secret mariadb-deployment

mariadb-certificates:
	rm -rf $(ROOT_DIR)/certs && mkdir $(ROOT_DIR)/certs
	cfssl gencert -initca $(ROOT_DIR)/ca-csr.json | cfssljson -bare $(ROOT_DIR)/certs/ca
	@mv $(ROOT_DIR)/certs/ca.pem $(ROOT_DIR)/certs/ca.crt
	@mv $(ROOT_DIR)/certs/ca-key.pem $(ROOT_DIR)/certs/ca.key
	cfssl gencert -ca=$(ROOT_DIR)/certs/ca.crt -ca-key=$(ROOT_DIR)/certs/ca.key \
		-config=$(ROOT_DIR)/config.json -profile=server \
		$(ROOT_DIR)/server-csr.json | cfssljson -bare $(ROOT_DIR)/certs/server
	@mv $(ROOT_DIR)/certs/server.pem $(ROOT_DIR)/certs/server.crt
	@mv $(ROOT_DIR)/certs/server-key.pem $(ROOT_DIR)/certs/server.key
	chmod 644 $(ROOT_DIR)/certs/*

mariadb-secret:
	@kubectl -n kamaji-system create secret generic mysql-config \
		--from-file=$(ROOT_DIR)/certs/ca.crt --from-file=$(ROOT_DIR)/certs/ca.key \
		--from-file=$(ROOT_DIR)/certs/server.key --from-file=$(ROOT_DIR)/certs/server.crt \
		--from-file=$(ROOT_DIR)/mysql-ssl.cnf \
		--from-literal=MYSQL_ROOT_PASSWORD=root \
		--dry-run=client -o yaml | kubectl apply -f -

mariadb-kine-secret:
	@\
		CA=$$(cat $(ROOT_DIR)/certs/ca.crt | base64 | tr -d '\n') \
		CRT=$$(cat $(ROOT_DIR)/certs/server.crt | base64 | tr -d '\n') \
		KEY=$$(cat $(ROOT_DIR)/certs/server.key | base64 | tr -d '\n') \
		ROOT_USERNAME=$$(echo -n root | base64) \
		ROOT_PASSWORD=$$(kubectl -n kamaji-system get secret mysql-config -o jsonpath='{.data.MYSQL_ROOT_PASSWORD}') \
		envsubst < $(ROOT_DIR)/../kine/secret.yaml | kubectl -n kamaji-system apply -f -

mariadb-deployment:
	@kubectl -n kamaji-system apply -f $(ROOT_DIR)/mariadb.yaml

mariadb-destroy:
	@kubectl delete -n kamaji-system -f $(ROOT_DIR)/mariadb.yaml --ignore-not-found
	@kubectl delete -n kamaji-system secret mysql-config --ignore-not-found
	@kubectl delete -n kamaji-system secret kine-secret --ignore-not-found
