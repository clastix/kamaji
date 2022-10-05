# Kamaji - Managed Kubernetes Service

Kamaji is a tool aimed to build and operate a Managed Kubernetes Service with a fraction of the operational burden.

Useful links:
- [Kamaji Github repository](https://github.com/clastix/kamaji)
- [Kamaji Documentation](https://github.com/clastix/kamaji/docs/)

## Requirements

* Kubernetes v1.22+
* Helm v3

# Installation

To install the Chart with the release name `kamaji`:

        helm upgrade --install --namespace kamaji-system --create-namespace clastix/kamaji

Show the status:

        helm status kamaji -n kamaji-system

Upgrade the Chart

        helm upgrade kamaji -n kamaji-system clastix/kamaji

Uninstall the Chart

        helm uninstall kamaji -n kamaji-system