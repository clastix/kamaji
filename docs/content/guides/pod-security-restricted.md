# Pod Security Standards

A Tenant Control Plane runs as a `Deployment` in the management cluster. When its namespace enforces the `restricted` [Pod Security Standard](https://kubernetes.io/docs/concepts/security/pod-security-standards/), for example on a management cluster shared by several teams, every container of that Deployment must comply, or the Pods are rejected.

Kamaji doesn't set any security context by default. This guide shows the fields to set on the `TenantControlPlane` so that its Pods pass the `restricted` profile.

## Example

The namespace enforces `restricted`:

```bash
kubectl create namespace tenant-a
kubectl label namespace tenant-a pod-security.kubernetes.io/enforce=restricted
```

The `TenantControlPlane` sets a security context for the Pod, for each control plane container, and for the Konnectivity server:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: tenant-a
  namespace: tenant-a
spec:
  kubernetes:
    version: v1.36.3
    kubelet:
      cgroupfs: systemd
  networkProfile:
    port: 6443
  controlPlane:
    service:
      serviceType: LoadBalancer
    deployment:
      replicas: 1
      podSecurityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
        fsGroup: 65532
        seccompProfile:
          type: RuntimeDefault
      containerSecurityContexts:
        apiServer:
          allowPrivilegeEscalation: false
          capabilities:
            drop: ["ALL"]
            add: ["NET_BIND_SERVICE"]
        controllerManager:
          allowPrivilegeEscalation: false
          capabilities:
            drop: ["ALL"]
        scheduler:
          allowPrivilegeEscalation: false
          capabilities:
            drop: ["ALL"]
  addons:
    coreDNS: {}
    kubeProxy: {}
    konnectivity:
      server:
        port: 8132
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop: ["ALL"]
```

- The upstream control plane images run as user `0` (root), so `runAsNonRoot: true` needs a numeric non-root `runAsUser`.
- `seccompProfile` is set once for the Pod and applies to every container.

## Why the API server keeps `NET_BIND_SERVICE`

The `kube-apiserver` binary in the upstream image is shipped with the file capability `cap_net_bind_service=ep`. When a container drops every capability and sets `allowPrivilegeEscalation: false`, the kernel refuses to execute a binary that carries a file capability outside the container's bounding set. The container then exits immediately with:

```
exec /usr/local/bin/kube-apiserver: operation not permitted
```

The `restricted` profile allows adding back exactly one capability, `NET_BIND_SERVICE`, which is what the `apiServer` security context above does. The controller manager and the scheduler binaries carry no file capability and run with every capability dropped.

## Verify a namespace

A server-side dry run shows which running Pods of a namespace would violate the profile, without changing the namespace:

```bash
kubectl label --dry-run=server --overwrite namespace tenant-a \
  pod-security.kubernetes.io/enforce=restricted
```

Every non-compliant Pod is listed as a warning. No warning means the Tenant Control Plane already complies.

## Limitations

- **Kine datastores.** With a MySQL, PostgreSQL or NATS `DataStore`, the Pod also runs the `kine` container and the `chmod` init container. `containerSecurityContexts` has no field for them, so these Tenant Control Planes can't comply with `restricted` yet. The example above uses an etcd `DataStore`.
- **Additional containers.** Containers added with `additionalContainers` or `additionalInitContainers` must set their own `securityContext`.
- **Resource quotas.** The default `RollingUpdate` strategy runs the old and the new Pod side by side during an update. In a namespace with a tight `ResourceQuota`, the new Pod can be rejected for exceeding the quota. Setting `spec.controlPlane.deployment.strategy.type: Recreate` avoids this, at the cost of a short API server downtime during updates.
