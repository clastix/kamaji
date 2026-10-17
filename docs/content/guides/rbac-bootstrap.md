# RBAC Bootstrap

A freshly provisioned Tenant Control Plane hands back an admin `kubeconfig`, but the identity in that `kubeconfig` has no RoleBindings in the Tenant cluster. Kamaji can create the initial `ClusterRoleBinding` for you, so a new cluster is usable straight away without an operator applying RBAC by hand.

## Enabling

RBAC bootstrap is opt-in: it applies only when the `bootstrap.rbac` stanza is present.

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: tenant-00
  namespace: default
spec:
  bootstrap:
    rbac:
      adminUsers:
        - kubernetes-admin
      adminGroups:
        - system:masters
  controlPlane:
    deployment:
      replicas: 2
    service:
      serviceType: LoadBalancer
  kubernetes:
    version: v1.32.0
    kubelet:
      cgroupfs: systemd
```

With the stanza present and `enabled` unset, bootstrap is performed: `enabled` defaults to `true`.

Kamaji creates a single `ClusterRoleBinding` in the Tenant cluster named `kamaji-<tenant-control-plane-name>-admin`, binding the `cluster-admin` `ClusterRole` to every user in `adminUsers` and every group in `adminGroups`.

Its name is derived only from the Tenant Control Plane name, so editing `adminUsers` or `adminGroups` updates the existing binding in place rather than leaving an orphan behind.

## Defaults

| Field | Default | Notes |
|---|---|---|
| `enabled` | `true` | Applies when the `bootstrap.rbac` stanza is present. |
| `adminUsers` | `["kubernetes-admin"]` | Matches the user in the generated admin `kubeconfig`. |
| `adminGroups` | `["system:masters"]` | The traditional Kubernetes administrators group. |

!!! note
    `system:masters` is hard-coded into the Kubernetes API Server authorizer and already carries full privileges without a binding. It is included in the defaults for completeness and discoverability; the binding that grants meaningful new access is the one for `adminUsers`.

## Checking the result

The name of the created binding is reported in the Tenant Control Plane status:

```bash
kubectl get tcp tenant-00 -o jsonpath='{.status.bootstrap.rbac.clusterRoleBinding.name}'
```

And from inside the Tenant cluster:

```bash
kubectl --kubeconfig tenant-00.kubeconfig get clusterrolebinding kamaji-tenant-00-admin -o yaml
```

## Restricting the granted access

The defaults grant `cluster-admin`. To bind a narrower set of identities, list only the users and groups you intend to be administrators:

```yaml
spec:
  bootstrap:
    rbac:
      adminUsers:
        - kubernetes-admin
      adminGroups: []
```

For anything beyond the initial administrator, manage RBAC in the Tenant cluster directly: this feature is deliberately limited to the bootstrap binding that makes the delivered `kubeconfig` usable.

## Disabling

Set `enabled` to `false` explicitly:

```yaml
spec:
  bootstrap:
    rbac:
      enabled: false
```

Kamaji removes the `ClusterRoleBinding` it created, identified by its own labels. Bindings created by anyone else are left untouched.

Omitting the `bootstrap.rbac` stanza entirely has the same effect: nothing is created, and any binding Kamaji previously created is cleaned up.

!!! warning
    Disabling RBAC bootstrap on a running Tenant cluster removes the binding that the generated admin `kubeconfig` relies on. Make sure another identity retains administrative access first, or you will lock yourself out of the Tenant cluster.
