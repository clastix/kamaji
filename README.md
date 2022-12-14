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
Kamaji turns any Kubernetes cluster into an _“admin cluster”_ to orchestrate other Kubernetes clusters called _“tenant clusters”_. What makes Kamaji special is that Control Planes of _“tenant clusters”_ are just regular pods running in the _“admin cluster”_ instead of dedicated Virtual Machines. This solution makes running control planes at scale cheaper and easier to deploy and operate.

![Architecture](docs/content/images/kamaji-light.png#gh-light-mode-only)
![Architecture](docs/content/images/kamaji-dark.png#gh-dark-mode-only)

## Getting started

Please refer to the [Getting Started guide](https://kamaji.clastix.io/getting-started/) to deploy a minimal setup of Kamaji on KinD.

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
- [x] Seamless migration between datastore with the same driver
- [ ] Automatic assigning of Tenant Control Plane to a datastore
- [ ] Autoscaling of Tenant Control Plane pods


## Documentation
Please, check the project's [documentation](https://kamaji.clastix.io/) for getting started with Kamaji.

## Contributions
Kamaji is Open Source with Apache 2 license and any contribution is welcome.

## Community
Join the [Kubernetes Slack Workspace](https://slack.k8s.io/) and the [`#kamaji`](https://kubernetes.slack.com/archives/C03GLTTMWNN) channel to meet end-users and contributors.
