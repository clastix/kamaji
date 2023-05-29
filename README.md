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

**Kamaji** deploys and operates **Kubernetes Control Plane** at scale with a fraction of the operational burden. Kamaji is special because the Control Plane components are running in a single pod instead of dedicated machines. This solution makes running multiple Control Planes cheaper and easier to deploy and operate.

![Architecture](docs/content/images/kamaji-light.png#gh-light-mode-only)
![Architecture](docs/content/images/kamaji-dark.png#gh-dark-mode-only)

## Features

- **Self Service Kubernetes:** leave users the freedom to self-provision their Kubernetes clusters according to the assigned boundaries.
- **Multi-cluster Management:** centrally manage multiple clusters from a single admin cluster. Happy SREs. 
- **Cheaper Control Planes:** place multiple control planes on a single node, instead of having three nodes for a single control plane.
- **Stronger Multi-Tenancy:** leave users to access the control plane with admin permissions while keeping them isolated at the infrastructure level.
- **Kubernetes Inception:** use Kubernetes to manage Kubernetes by re-using all the Kubernetes goodies you already know and love.
- **Full APIs compliant:** all clusters are CNCF compliant built with upstream Kubernetes binaries

## Roadmap

- [x] Dynamic address on Load Balancer
- [x] Zero Downtime Tenant Control Plane upgrade
- [x] Join worker nodes from anywhere
- [x] Alternative datastore MySQL and PostgreSQL
- [x] Pool of multiple datastores
- [x] Seamless migration between datastores
- [ ] Automatic assignment to a datastore
- [ ] Autoscaling of Tenant Control Plane
- [ ] Provisioning through Cluster APIs
- [ ] Terraform provider
- [ ] Custom Prometheus metrics for monitoring and alerting


## Documentation
Please, check the project's [documentation](https://kamaji.clastix.io/) for getting started with Kamaji.

## Contributions
Kamaji is Open Source with Apache 2 license and any contribution is welcome.

## Community
Join the [Kubernetes Slack Workspace](https://slack.k8s.io/) and the [`#kamaji`](https://kubernetes.slack.com/archives/C03GLTTMWNN) channel to meet end-users and contributors.
