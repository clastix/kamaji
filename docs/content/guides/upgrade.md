# Tenant Cluster Upgrade
The process of upgrading a _“tenant cluster”_ consists in two steps:

1. Upgrade the Tenant Control Plane
2. Upgrade of Tenant Worker Nodes

## Upgrade of Tenant Control Plane
You should patch the `TenantControlPlane.spec.kubernetes.version` custom resource with a new compatible value according to the [Version Skew Policy](https://kubernetes.io/releases/version-skew-policy/).

> Note: during the upgrade, a new ReplicaSet of Tenant Control Plane pod will be created, so make sure you have at least two pods to avoid service disruption.

## Upgrade of Tenant Worker Nodes
As currently Kamaji is not providing any helpers for Tenant Worker Nodes, you should make sure to upgrade them manually, for example, with the help of `kubeadm`. We have in roadmap, the Cluster APIs support so that you can upgrade _“tenant clusters”_ in a fully declarative way.