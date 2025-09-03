# ReadOnly mode

The **ReadOnly** mode in Kamaji allows operators to lock a Tenant Control Plane (TCP) in a state where  no further modifications can be made by its tenants. This feature ensures consistency during maintenance, migrations, incident recovery, or when freezing workloads for auditing and compliance purposes.

## Enabling ReadOnly Mode

You can enable ReadOnly mode by setting the boolean field `TenantControlPlane.spec.readOnly` to `true`.

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: my-control-plane
spec:
  readOnly: true
```

Once applied, the Tenant Control Plane will switch into `ReadOnly` status.

## Monitoring the status

You can verify the status of your Tenant Control Plane with `kubectl get tcp`:

```json
$: kubectl get tcp k8s-133
NAME      VERSION   INSTALLED VERSION   STATUS     CONTROL-PLANE ENDPOINT   KUBECONFIG                 DATASTORE   AGE
k8s-133   v1.33.0   v1.33.0             ReadOnly   172.18.255.100:6443      k8s-133-admin-kubeconfig   default     50d
```

The `STATUS` field will display `ReadOnly` when the mode is active.

## How it works

When a Tenant Control Plane is set to `ReadOnly`, camaji creates a `ValidatingWebhookConfiguration` in the Tenant Cluster:

```
$: kubectl get validatingwebhookconfigurations
NAME              WEBHOOKS   AGE
kamaji-readonly   2          59m
```

The webhook intercepts all API requests to the Tenant Control Plane and denies any attempts to modify resources.

As a result, all changes initiated by tenants (including `kubectl apply`, `kubectl delete`, or CRD updates) will be blocked.

!!! note "When dealing with Operators and Controller"
    All actions are intercepted by such webhook: if a Pod must be rescheduled, the webhook will deny it.
    This behaviour could bring to outages of deployed workloads.

## Behaviour in ReadOnly Mode

If a tenant user tries to modify a resource, the following error is returned:

```
Error from server (Forbidden): admission webhook "catchall.readonly.kamaji.clastix.io" denied the request: the current Control Plane is in ReadOnly mode, all the changes are blocked: removing the webhook may lead to an inconsistent state upon its completion
```

This guarantees the cluster remains in a frozen, consistent state, preventing partial updates or drift.

## Use Cases

Typical scenarios where ReadOnly mode is useful:

- **Planned Maintenance**: freeze workloads before performing upgrades or infrastructure changes.
- **Disaster Recovery**: lock the Tenant Control Plane to prevent accidental modifications during incident handling.
- **Auditing & Compliance**: ensure workloads cannot be altered during a compliance check or certification process.

!!! note "Migrating the DataStore"
    On a similar manner, when migrating a Tenant Control Plane to a different store, similar enforcement is put in place.
    This is managed automatically by Kamaji and there's no need to toggle on and off the ReadOnly mode.
