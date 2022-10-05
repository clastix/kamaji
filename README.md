# Kamaji

<p align="left">
  <img src="https://img.shields.io/github/license/clastix/kamaji"/>
  <img src="https://img.shields.io/github/go-mod/go-version/clastix/kamaji"/>
  <a href="https://github.com/clastix/kamaji/releases">
    <img src="https://img.shields.io/github/v/release/clastix/kamaji"/>
  </a>
</p>

**Kamaji** deploys and operates **Kubernetes** at scale with a fraction of the operational burden.

<p align="center" style="padding: 6px 6px">
  <img src="assets/kamaji-logo.png" />
</p>

## Why we are building it?
Global hyper-scalers are leading the Managed Kubernetes space, while other cloud providers, as well as large corporations, are struggling to offer the same experience to their DevOps teams because of the lack of the right tools. Also, current Kubernetes solutions are mainly designed with an enterprise-first approach and they are too costly when deployed at scale.

**Kamaji** aims to solve these pains by leveraging multi-tenancy and simplifying how to run multiple control planes on the same infrastructure with a fraction of the operational burden.

## How it works
Kamaji turns any Kubernetes cluster into an _“admin cluster”_ to orchestrate other Kubernetes clusters called _“tenant clusters”_. What makes Kamaji special is that Control Planes of _“tenant clusters”_ are just regular pods running in the _“admin cluster”_ instead of dedicated Virtual Machines. This solution makes running control planes at scale cheaper and easier to deploy and operate. View [Core Concepts](./docs/concepts.md) for a deeper understanding of principles behind Kamaji's design.

<p align="center">
  <img src="assets/kamaji-light.png#gh-light-mode-only" />
</p>

<p align="center">
  <img src="assets/kamaji-dark.png#gh-dark-mode-only" />
</p>

All the tenant clusters built with Kamaji are fully compliant [CNCF Certified Kubernetes](https://www.cncf.io/certification/software-conformance/) and are compatible with the standard toolchains everybody knows and loves.

<p align="center" style="padding: 6px 6px">
  <img src="https://raw.githubusercontent.com/cncf/artwork/master/projects/kubernetes/certified-kubernetes/versionless/color/certified-kubernetes-color.png" width="100" />
</p>

## Getting started

Please refer to the [Getting Started guide](./docs/getting-started-with-kamaji.md) to deploy a minimal setup of Kamaji on KinD.

## Use cases
Kamaji project has been initially started as a solution for actual and common problems such as minimizing the Total Cost of Ownership while running Kubernetes at large scale. However, it can open a wider range of use cases.

Here are a few:

- **Managed Kubernetes:** enable companies to provide Cloud Native Infrastructure with ease by introducing a strong separation of concerns between management and workloads. Centralize clusters management, monitoring, and observability by leaving developers to focus on applications, increase productivity and reduce operational costs.
- **Kubernetes as a Service:** provide Kubernetes clusters in a self-service fashion by running management and workloads on different infrastructures with the option of Bring Your Own Device, BYOD.
- **Control Plane as a Service:** provide multiple Kubernetes control planes running on top of a single Kubernetes cluster. Tenants who use namespaces based isolation often still need access to cluster wide resources like Cluster Roles, Admission Webhooks, or Custom Resource Definitions.
- **Edge Computing:** distribute Kubernetes workloads across edge computing locations without having to manage multiple clusters across various providers. Centralize management of hundreds of control planes while leaving workloads to run isolated on their own dedicated infrastructure.
- **Cluster Simulation:** check new Kubernetes API or experimental flag or a new tool without impacting production operations. Kamaji will let you simulate such things in a safe and controlled environment.
- **Workloads Testing:** check the behaviour of your workloads on different and multiple versions of Kubernetes with ease by deploying multiple Control Planes in a single cluster.

## Features

- **Self Service Kubernetes:** leave users the freedom to self-provision their Kubernetes clusters according to the assigned boundaries.
- **Multi-cluster Management:** centrally manage multiple tenant clusters from a single admin cluster. Happy SREs. 
- **Cheaper Control Planes:** place multiple tenant control planes on a single node, instead of having three nodes for a single control plane.
- **Stronger Multi-Tenancy:** leave tenants to access the control plane with admin permissions while keeping the tenant isolated at the infrastructure level.
- **Kubernetes Inception:** use Kubernetes to manage Kubernetes by re-using all the Kubernetes goodies you already know and love.
- **Full APIs compliant:** tenant clusters are fully CNCF compliant built with upstream Kubernetes binaries. A user does not see differences between a Kamaji provisioned cluster and a dedicated cluster.

## Roadmap

- [ ] Benchmarking and stress-test
- [x] Support for dynamic address allocation on native Load Balancer
- [x] Zero Downtime Tenant Control Plane upgrade
- [x] `konnectivity` integration
- [ ] Provisioning of Tenant Control Plane through Cluster APIs
- [ ] Terraform provider
- [ ] Custom Prometheus metrics for monitoring and alerting
- [x] `kine` integration for MySQL as datastore
- [x] `kine` integration for PostgreSQL as datastore
- [x] Pool of multiple datastores
- [ ] Automatic assigning of Tenant Control Plane to a datastore
- [ ] Autoscaling of Tenant Control Plane pods


## Documentation
Please, check the project's [documentation](./docs/) for getting started with Kamaji.

## Contributions
Kamaji is Open Source with Apache 2 license and any contribution is welcome.

## Community
Join the [Kubernetes Slack Workspace](https://slack.k8s.io/) and the [`#kamaji`](https://kubernetes.slack.com/archives/C03GLTTMWNN) channel to meet end-users and contributors.

## FAQs
Q. What does Kamaji means?

A. Kamaji is named as the character _Kamaji_ from the Japanese movie [_Spirited Away_](https://en.wikipedia.org/wiki/Spirited_Away).

Q. Is Kamaji another Kubernetes distribution?

A. No, Kamaji is a Kubernetes Operator you can install on top of any Kubernetes cluster to provide hundreds of managed Kubernetes clusters as a service. We tested Kamaji on vanilla Kubernetes 1.22+, KinD, and Azure AKS. We expect it to work smoothly on other Kubernetes distributions. The tenant clusters made with Kamaji are conformant CNCF Kubernetes clusters as we leverage on [`kubeadm`](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/).

Q. Is it safe to run Kubernetes control plane components in a pod instead of dedicated virtual machines?

A. Yes, the tenant control plane components are packaged in the same way they are running in bare metal or virtual nodes. We leverage the `kubeadm` code to set up the control plane components as they were running on their own server. The unchanged images of upstream `kube-apiserver`, `kube-scheduler`, and `kube-controller-manager` are used.

Q. You already provide a Kubernetes multi-tenancy solution with [Capsule](https://capsule.clastix.io). Why does Kamaji matter?

A. A multi-tenancy solution, like Capsule shares the Kubernetes control plane among all tenants keeping tenant namespaces isolated by policies. While the solution is the right choice by balancing between features and ease of usage, there are cases where a tenant user requires access to the control plane, for example, when a tenant requires to manage CRDs on his own. With Kamaji, you can provide cluster admin permissions to the tenant.

Q. Well you convinced me, how to get a try?

A. It is possible to get started with Kamaji on a laptop with [KinD](./docs/getting-started-with-kamaji.md) installed.
