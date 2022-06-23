mariadb_path := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

.PHONY: mariadb mariadb-certificates mariadb-secrets

mariadb: mariadb-certificates mariadb-secrets mariadb-deployment

mariadb-certificates:
	rm -rf $(mariadb_path)/certs && mkdir $(mariadb_path)/certs
	cfssl gencert -initca $(mariadb_path)/ca-csr.json | cfssljson -bare $(mariadb_path)/certs/ca
	@mv $(mariadb_path)/certs/ca.pem $(mariadb_path)/certs/ca.crt
	@mv $(mariadb_path)/certs/ca-key.pem $(mariadb_path)/certs/ca.key
	cfssl gencert -ca=$(mariadb_path)/certs/ca.crt -ca-key=$(mariadb_path)/certs/ca.key \
		-config=$(mariadb_path)/config.json -profile=server \
		$(mariadb_path)/server-csr.json | cfssljson -bare $(mariadb_path)/certs/server
	@mv $(mariadb_path)/certs/server.pem $(mariadb_path)/certs/server.crt
	@mv $(mariadb_path)/certs/server-key.pem $(mariadb_path)/certs/server.key
	chmod 644 $(mariadb_path)/certs/*

mariadb-secrets:
	@kubectl -n kamaji-system create secret generic mysql-config \
		--from-file=$(mariadb_path)/certs/ca.crt --from-file=$(mariadb_path)/certs/ca.key \
		--from-file=$(mariadb_path)/certs/server.key --from-file=$(mariadb_path)/certs/server.crt \
		--from-file=$(mariadb_path)/mysql-ssl.cnf \
		--from-literal=MYSQL_ROOT_PASSWORD=root

mariadb-deployment:
	@kubectl -n kamaji-system apply -f $(mariadb_path)/mariadb.yaml

destroy:
	@kubectl delete -n kamaji-system -f $(mariadb_path)/mariadb.yaml
	@kubectl delete -n kamaji-system secret mysql-config
