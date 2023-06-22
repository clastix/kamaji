# Tenant Cluster Upgrade
The process of upgrading a _“tenant cluster”_ consists in two steps:

1. Upgrade the Tenant Control Plane
2. Upgrade of Tenant Worker Nodes

## Upgrade of Tenant Control Plane
You should patch the `TenantControlPlane.spec.kubernetes.version` custom resource with a new compatible value according to the [Version Skew Policy](https://kubernetes.io/releases/version-skew-policy/).

During the upgrade, a new ReplicaSet of Tenant Control Plane pod will be created, so make sure you have enough replicas to avoid service disruption. Also make sure you have the Rolling Update strategy properly configured:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: tenant-00
spec:
  controlPlane:
    deployment:
      replicas: 3
      strategy:
        rollingUpdate:
          maxSurge: 1
          maxUnavailable: 1
        type: RollingUpdate
...
```

## Upgrade of Tenant Worker Nodes

As currently Kamaji is not providing any helpers for Tenant Worker Nodes, you should make sure to upgrade them manually, for example, with the help of `kubeadm`.
Refer to the official [documentation](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-upgrade/#upgrade-worker-nodes).

Kamaji is offering a [Cluster API Control Plane provider](https://github.com/clastix/cluster-api-control-plane-provider-kamaji), thus integrating with the Kubernetes clusters declarative management approach.
You can refer to the official [Cluster API documentation](https://cluster-api.sigs.k8s.io/).
