kind_path := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

include ../etcd/Makefile

.PHONY: kind ingress-nginx

.DEFAULT_GOAL := kamaji

prometheus-stack:
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
	helm repo update
	helm install prometheus-stack --create-namespace -n monitoring prometheus-community/kube-prometheus-stack

reqs: kind ingress-nginx cert-manager

cert-manager:
	@kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.10.1/cert-manager.yaml

kamaji: reqs
	helm install kamaji --create-namespace -n kamaji-system $(kind_path)/../../charts/kamaji

destroy: kind/destroy etcd-certificates/cleanup

kind:
	@kind create cluster --config $(kind_path)/kind-kamaji.yaml

kind/destroy:
	@kind delete cluster --name kamaji

ingress-nginx: ingress-nginx-install

ingress-nginx-install:
	kubectl apply -f $(kind_path)/nginx-deploy.yaml

kamaji-kind-worker-join:
	$(kind_path)/join-node.bash
