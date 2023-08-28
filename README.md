# Kamaji

<p align="left">
  <img src="https://img.shields.io/github/license/clastix/kamaji"/>
  <img src="https://img.shields.io/github/go-mod/go-version/clastix/kamaji"/>
  <a href="https://github.com/clastix/kamaji/releases">
    <img src="https://img.shields.io/github/v/release/clastix/kamaji"/>
    <img src="https://goreportcard.com/badge/github.com/clastix/kamaji">
  </a>
</p>

![Logo](assets/logo-black.png#gh-light-mode-only)
![Logo](assets/logo-white.png#gh-dark-mode-only)

**Kamaji** is a **Kubernetes Control Plane Manager**. It operates Kubernetes at scale with a fraction of the operational burden. Kamaji is special because the Control Plane components are running inside pods instead of dedicated machines. This solution makes running multiple Control Planes cheaper and easier to deploy and operate.

<img src="docs/content/images/architecture.png"  width="600">

## Main Features

- **Multi-cluster Management:** centrally manage multiple Kubernetes clusters from a single Management Cluster. 
- **High-density Control Plane:** place multiple control planes on the same infrastructure, instead of having dedicated machines for each control plane.
- **Strong Multi-tenancy:** leave users to access the control plane with admin permissions while keeping them isolated at the infrastructure level.
- **Kubernetes Inception:** use Kubernetes to manage Kubernetes cluster with automation, high-availability, fault tolerance, and autoscaling out of the box. 
- **Bring Your Own Device:** keep the control plane isolated from data plane. Worke nodes can join and run consistently everywhere: cloud, edge, and data-center.
- **Full CNCF compliant:** all clusters are built with upstream Kubernetes binaries, resulting in full CNCF compliant Kubernetes clusters.

## Roadmap

- [x] Dynamic address on Load Balancer
- [x] Zero Downtime Tenant Control Plane upgrade
- [x] Join worker nodes from anywhere
- [x] Alternative datastore MySQL and PostgreSQL
- [x] Pool of multiple datastores
- [x] Seamless migration between datastores
- [ ] Automatic assignment to a datastore
- [ ] Autoscaling of Tenant Control Plane
- [x] Provisioning through Cluster APIs
- [ ] Terraform provider
- [ ] Custom Prometheus metrics


## Documentation
Please, check the project's [documentation](https://kamaji.clastix.io/) for getting started with Kamaji.

## Contributions
Kamaji is Open Source with Apache 2 license and any contribution is welcome. Open an issue or suggest an enhancement on the GitHub [project's page](https://github.com/clastix/kamaji). Join the [Kubernetes Slack Workspace](https://slack.k8s.io/) and the [`#kamaji`](https://kubernetes.slack.com/archives/C03GLTTMWNN) channel to meet end-users and contributors.
