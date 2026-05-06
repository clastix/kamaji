# Restricting the namespaces watched by Kamaji

By default Kamaji watches every namespace of the management cluster for
`TenantControlPlane` resources and the dependent objects it owns
(`Secret`, `ConfigMap`, `Deployment`, `Service`, `Ingress`, ...).

In multi-team or multi-tenant management clusters this is sometimes too broad.
You can opt-in to a smaller watch surface with the `--watch-namespaces` flag,
which leverages controller-runtime's per-namespace cache.

## The `--watch-namespaces` flag

```text
--watch-namespaces=team-a,team-b
```

- Accepts a comma-separated list (or repeated `--watch-namespaces=...` flags).
- Each entry must be a valid Kubernetes namespace identifier (DNS-1123
  label); invalid values cause the operator to fail fast at startup rather
  than surface opaque watch errors at runtime.
- When **omitted or empty**, Kamaji keeps the default cluster-wide behaviour.
- When set, namespaced informers only watch the listed namespaces.
- The operator's own install namespace is always added to the watch set
  implicitly: `TenantControlPlane` migration `Job`s live there and would not
  be reconciled otherwise.
- The flag is honoured by both the main controller and the optional
  **kubeconfig-generator** deployment; the Helm chart threads the same
  `watchNamespaces` value into both.

## What is and is not affected

Namespace scoping only constrains **namespaced** resources. Cluster-scoped
resources continue to be cached cluster-wide and keep working unchanged:

| Resource                            | Scope          | Honours `--watch-namespaces`? |
| ----------------------------------- | -------------- | ----------------------------- |
| `TenantControlPlane`                | Namespaced     | Yes                           |
| `Secret`, `ConfigMap`, `Deployment`, `Service`, `Ingress` (TCP children) | Namespaced | Yes |
| Migration `Job` in the install ns   | Namespaced     | Implicitly always included    |
| `DataStore`                         | Cluster        | No (always cluster-wide)      |
| `ValidatingWebhookConfiguration`    | Cluster        | No                            |
| `ClusterRole`, `ClusterRoleBinding` | Cluster        | No                            |
| Soot controllers (kubeadm phases, kube-proxy, CoreDNS, ...) | N/A | These run against the **tenant** cluster's API server with their own cache, so the management-cluster scoping does not apply to them. |

## Caveats

- **Gateway API**: when a `TenantControlPlane` references a `Gateway` that
  lives in a namespace **outside** the watch set, the operator will not be able
  to read it from the cache. Add every namespace hosting referenced
  `Gateway` resources to `--watch-namespaces`.
- **RBAC**: scoping only affects what the cache subscribes to, not what the
  Kubernetes API authorises. The default Helm chart still installs a
  `ClusterRole` to keep upgrades and cluster-scoped reconciliations safe.
- **Scaling**: controller-runtime allocates one informer per `(namespace,
  kind)` pair when `DefaultNamespaces` is set. Kamaji watches roughly seven
  namespaced kinds in the management cluster (`TenantControlPlane` plus its
  owned `Secret`, `ConfigMap`, `Deployment`, `Service`, `Ingress`, `Job`).
  Listing a few dozen namespaces is comfortable; listing several hundred
  multiplies the number of `LIST/WATCH` connections and goroutines and may
  exceed client-go QPS defaults — at that point the cluster-wide single
  informer is cheaper. Prefer per-cluster Kamaji instances over very long
  watch lists.

## Helm

The chart exposes a top-level `watchNamespaces` value that maps to the flag:

```yaml
# values.yaml
watchNamespaces:
  - team-a
  - team-b
```

Or via `--set` on the command line:

```bash
helm upgrade --install kamaji clastix/kamaji \
  --namespace kamaji-system --create-namespace \
  --set 'watchNamespaces={team-a,team-b}'
```

Leaving the list empty (the default) renders no `--watch-namespaces` argument
and preserves the cluster-wide behaviour.
