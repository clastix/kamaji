kind_path := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

include ../etcd/Makefile

.PHONY: kind ingress-nginx kamaji-kind-worker-build

.DEFAULT_GOAL := kamaji

prometheus-stack:
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
	helm repo update
	helm install prometheus-stack --create-namespace -n monitoring prometheus-community/kube-prometheus-stack

kamaji: kind ingress-nginx etcd-cluster

destroy: kind/destroy etcd-certificates/cleanup

kind:
	@kind create cluster --config $(kind_path)/kind-kamaji.yaml

kind/destroy:
	@kind delete cluster --name kamaji

ingress-nginx: ingress-nginx-install

ingress-nginx-install:
	kubectl apply -f $(kind_path)/nginx-deploy.yaml

kamaji-kind-worker-build:
	docker build -f $(kind_path)/kamaji-kind-worker.dockerfile -t quay.io/clastix/kamaji-kind-worker:$${WORKER_VERSION:-latest} .

kamaji-kind-worker-push: kamaji-kind-worker-build
	docker push quay.io/clastix/kamaji-kind-worker:$${WORKER_VERSION:-latest}

kamaji-kind-worker-join:
	$(kind_path)/join-node.bash
