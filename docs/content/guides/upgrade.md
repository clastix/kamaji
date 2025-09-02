# Tenant Cluster Upgrade

Upgrading a _Tenant Cluster_ consists of two main steps:

1. Upgrade the Tenant Control Plane
2. Upgrade the Tenant Worker Nodes

---

## Upgrade of Tenant Control Plane

The version of the Tenant Control Plane is managed by updating the `TenantControlPlane.spec.kubernetes.version` field.  
You should patch this field with a new compatible value according to the [Kubernetes Version Skew Policy](https://kubernetes.io/releases/version-skew-policy/).

### Default Upgrade Strategy (Blue/Green)

By default, when you upgrade a `TenantControlPlane`, Kamaji applies a **Blue/Green deployment** strategy.

- `maxSurge: 100%`: all new control plane Pods are created at once.
- `maxUnavailable: 0`: existing Pods remain running until the new Pods are ready.

This ensures that the new ReplicaSet of Tenant Control Plane Pods comes up alongside the existing ones,
minimising disruption and guaranteeing immediate failover.

This approach provides some pros, such as a fast upgrade, and a minimal downtime, since existing Pods remain until the new ones are healthy.

However, all new Pods start simultaneously, which may _overload communications with the DataStore_,
and it requires sufficient cluster resources to host double the number of control plane Pods temporarily.

### Alternative: Rolling Upgrade Strategy

In environments with _constrained resources_, or where DataStore connections must be protected from sudden load spikes,
you should configure a **Rolling Upgrade** strategy.

This approach ensures that only a subset of Pods is replaced at a time,
gradually rolling out the new version without stressing the infrastructure.

Example configuration:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: tenant-00
  labels:
    tenant.clastix.io: tenant-00
spec:
  controlPlane:
    deployment:
      replicas: 3
      strategy:
        type: RollingUpdate
        rollingUpdate:
          maxSurge: 1
          maxUnavailable: 1
```

## Upgrade of Tenant Worker Nodes

As currently Kamaji is not providing any helpers for Tenant Worker Nodes, you should make sure to upgrade them manually, for example, with the help of `kubeadm`.
Refer to the official [documentation](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-upgrade/#upgrade-worker-nodes).

Kamaji is offering a [Cluster API Control Plane provider](https://github.com/clastix/cluster-api-control-plane-provider-kamaji), thus integrating with the Kubernetes clusters declarative management approach.
You can refer to the official [Cluster API documentation](https://cluster-api.sigs.k8s.io/).
