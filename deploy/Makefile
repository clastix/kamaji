include etcd/Makefile

deploy_path := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

.DEFAULT_GOAL := kamaji

.PHONY: etcd-cluster
reqs: etcd-cluster

.PHONY: kamaji
kamaji: reqs
	@kubectl apply -f $(deploy_path)/../../config/install.yaml

.PHONY: destroy
destroy: etcd-certificates/cleanup
	@kubectl delete -f $(deploy_path)/../../config/install.yaml
