etcd_path := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

.PHONY: etcd-cluster etcd-certificates etcd-cluster-install etcd-enable-multitenancy

etcd-cluster: etcd-certificates etcd-cluster-install etcd-cluster-healthcheck etcd-enable-multitenancy

etcd-certificates:
	rm -rf $(etcd_path)/certs && mkdir $(etcd_path)/certs
	cfssl gencert -initca $(etcd_path)/ca-csr.json | cfssljson -bare $(etcd_path)/certs/ca
	mv $(etcd_path)/certs/ca.pem $(etcd_path)/certs/ca.crt
	mv $(etcd_path)/certs/ca-key.pem $(etcd_path)/certs/ca.key
	cfssl gencert -ca=$(etcd_path)/certs/ca.crt -ca-key=$(etcd_path)/certs/ca.key \
		-config=$(etcd_path)/config.json \
		-profile=peer-authentication $(etcd_path)/peer-csr.json | cfssljson -bare $(etcd_path)/certs/peer
	cfssl gencert -ca=$(etcd_path)/certs/ca.crt -ca-key=$(etcd_path)/certs/ca.key \
		-config=$(etcd_path)/config.json \
		-profile=peer-authentication $(etcd_path)/server-csr.json | cfssljson -bare $(etcd_path)/certs/server
	cfssl gencert -ca=$(etcd_path)/certs/ca.crt -ca-key=$(etcd_path)/certs/ca.key \
		-config=$(etcd_path)/config.json \
		-profile=client-authentication $(etcd_path)/root-client-csr.json | cfssljson -bare $(etcd_path)/certs/root-client	

etcd-cluster-install:
	@kubectl create namespace kamaji-system --dry-run=client -o yaml | kubectl apply -f -
	@kubectl -n kamaji-system apply -f $(etcd_path)/etcd-cluster.yaml
	@kubectl -n kamaji-system create secret generic etcd-certs \
		--from-file=$(etcd_path)/certs/ca.crt \
		--from-file=$(etcd_path)/certs/ca.key \
		--from-file=$(etcd_path)/certs/peer-key.pem --from-file=$(etcd_path)/certs/peer.pem \
		--from-file=$(etcd_path)/certs/server-key.pem --from-file=$(etcd_path)/certs/server.pem
	@kubectl -n kamaji-system create secret tls root-client-certs \
		--key=$(etcd_path)/certs/root-client-key.pem \
		--cert=$(etcd_path)/certs/root-client.pem

etcd-cluster-healthcheck:
	@sleep 20
	@echo "Wait the etcd instances discover each other and the cluster is formed"
	@kubectl wait pod --for=condition=ready -n kamaji-system -l app=etcd --timeout=120s
	@kubectl -n kamaji-system apply -f $(etcd_path)/etcd-client.yaml
	@sleep 20
	@echo -n "Checking endpoint's health..."
	@kubectl -n kamaji-system exec etcd-root-client -- /bin/bash -c \
		"etcdctl endpoint health 1>/dev/null 2>/dev/null; until [ \$$? -eq 0 ]; do sleep 10; printf "."; etcdctl endpoint health 1>/dev/null 2>/dev/null; done;"
	@echo -n "etcd cluster's health:\n"
	@kubectl -n kamaji-system exec etcd-root-client -- /bin/bash -c \
		"etcdctl endpoint health"
	@echo -n "Waiting for all members..."
	@kubectl -n kamaji-system exec etcd-root-client -- /bin/bash -c \
		"until [ \$$(etcdctl member list 2>/dev/null | wc -l) -eq 3 ]; do sleep 10; printf '.'; done;"
	@echo -n "etcd's members:\n"
	@kubectl -n kamaji-system exec etcd-root-client -- /bin/bash -c \
		"etcdctl member list -w table"


etcd-enable-multitenancy:
	kubectl -n kamaji-system exec etcd-root-client -- etcdctl user add --no-password=true root
	kubectl -n kamaji-system exec etcd-root-client -- etcdctl role add root
	kubectl -n kamaji-system exec etcd-root-client -- etcdctl user grant-role root root
	kubectl -n kamaji-system exec etcd-root-client -- etcdctl auth enable

etcd-certificates/cleanup:
	@rm -rf $(etcd_path)/certs
