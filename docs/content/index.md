# Kamaji
**Kamaji** deploys and operates Kubernetes at scale with a fraction of the operational burden. 

## How it works
Kamaji turns any Kubernetes cluster into an _“admin cluster”_ to orchestrate other Kubernetes clusters called _“tenant clusters”_. What makes Kamaji special is that Control Planes of _“tenant clusters”_ are just regular pods running in the _“admin cluster”_ instead of dedicated Virtual Machines. This solution makes running control planes at scale cheaper and easier to deploy and operate. View [Concepts](concepts.md) for a deeper understanding of principles behind Kamaji's design.

![Architecture](images/kamaji-light.png#gh-light-mode-only)
![Architecture](images/kamaji-dark.png#gh-dark-mode-only)

All the tenant clusters built with Kamaji are fully compliant [CNCF Certified Kubernetes](https://www.cncf.io/certification/software-conformance/) and are compatible with the standard toolchains everybody knows and loves.

<p align="center" style="padding: 6px 6px">
  <img src="https://raw.githubusercontent.com/cncf/artwork/master/projects/kubernetes/certified-kubernetes/versionless/color/certified-kubernetes-color.png" width="200" />
</p>

## Features

- **Self Service Kubernetes:** leave users the freedom to self-provision their Kubernetes clusters according to the assigned boundaries.
- **Multi-cluster Management:** centrally manage multiple tenant clusters from a single admin cluster. Happy SREs. 
- **Cheaper Control Planes:** place multiple tenant control planes on a single node, instead of having three nodes for a single control plane.
- **Stronger Multi-Tenancy:** leave tenants to access the control plane with admin permissions while keeping the tenant isolated at the infrastructure level.
- **Kubernetes Inception:** use Kubernetes to manage Kubernetes by re-using all the Kubernetes goodies you already know and love.
- **Full APIs compliant:** tenant clusters are fully CNCF compliant built with upstream Kubernetes binaries. A user does not see differences between a Kamaji provisioned cluster and a dedicated cluster.

## Getting started

Please refer to the [Getting Started guide](getting-started.md) to deploy a minimal setup of Kamaji on [KinD](https://kind.sigs.k8s.io/).

## Open Source
Kamaji is Open Source with Apache 2 license and any contribution is welcome. Open an issue or suggest an enhancement on the GitHub [project's page](https://github.com/clastix/kamaji). Join the [Kubernetes Slack Workspace](https://slack.k8s.io/) and the [`#kamaji`](https://kubernetes.slack.com/archives/C03GLTTMWNN) channel to meet end-users and contributors.

## FAQs
Q. What does Kamaji mean?

A. Kamaji is named as the character _Kamaji_ from the Japanese movie [_Spirited Away_](https://en.wikipedia.org/wiki/Spirited_Away).

Q. Is Kamaji another Kubernetes distribution?

A. No, Kamaji is a Kubernetes Operator you can install on top of any Kubernetes cluster to provide hundreds or thousands of managed Kubernetes clusters as a service. We tested Kamaji on vanilla Kubernetes 1.22+, KinD, and Azure AKS. We expect it to work smoothly on other Kubernetes distributions. The tenant clusters made with Kamaji are conformant CNCF Kubernetes clusters as we leverage [`kubeadm`](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/).

Q. Is it safe to run Kubernetes control plane components in a pod instead of dedicated virtual machines?

A. Yes, the tenant control plane components are packaged in the same way they are running in bare metal or virtual nodes. We leverage the `kubeadm` code to set up the control plane components as they were running on their own server. The unchanged images of upstream `kube-apiserver`, `kube-scheduler`, and `kube-controller-manager` are used.

Q. You already provide a Kubernetes multi-tenancy solution with [Capsule](https://capsule.clastix.io). Why does Kamaji matter?

A. A multi-tenancy solution, like Capsule shares the Kubernetes control plane among all tenants keeping tenant namespaces isolated by policies. While the solution is the right choice by balancing between features and ease of usage, there are cases where a tenant user requires access to the control plane, for example, when a tenant requires to manage CRDs on his own. With Kamaji, you can provide cluster admin permissions to the tenant.

Q. Well you convinced me, how to get a try?

A. It is possible to get started with Kamaji on a laptop with [KinD](getting-started.md) installed.
