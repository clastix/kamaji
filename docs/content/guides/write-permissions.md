# Write Permissions

Using the _Write Permissions_ section, operators can limit write operations for a given Tenant Control Plane,
where no further write actions can be made by its tenants.

This feature ensures consistency during maintenance, migrations, incident recovery, quote enforcement,
or when freezing workloads for auditing and compliance purposes.

Write Operations can limit the following actions:

- Create
- Update
- Delete

By default, all write operations are allowed.

## Enabling a Read-Only mode

You can enable ReadOnly mode by setting all the boolean fields of `TenantControlPlane.spec.writePermissions` to `true`.

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: my-control-plane
spec:
  writePermissions:
    blockCreate: true
    blockUpdate: true
    blockDelete: true
```

Once applied, the Tenant Control Plane will switch into `WriteLimited` status.

## Enforcing a quota mode

If your Tenant Control Plane has a Datastore quota, this feature allows freezing write and update operations,
but still allowing its tenants to perform a clean-up by deleting exceeding resources.

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: my-control-plane
spec:
  writePermissions:
    blockCreate: true
    blockUpdate: true
    blockDelete: false
```

!!! note "Datastore quota"
    Kamaji does **not** enforce storage quota for a given Tenant Control Plane:
    you have to implement it according to your business logic.

## Monitoring the status

You can verify the status of your Tenant Control Plane with `kubectl get tcp`:

```json
$: kubectl get tcp k8s-133
NAME      VERSION   INSTALLED VERSION   STATUS         CONTROL-PLANE ENDPOINT   KUBECONFIG                 DATASTORE   AGE
k8s-133   v1.33.0   v1.33.0             WriteLimited   172.18.255.100:6443      k8s-133-admin-kubeconfig   default     50d
```

The `STATUS` field will display `WriteLimited` when write permissions are limited.

## How it works

When a Tenant Control Plane write status is _limited_, Kamaji creates a `ValidatingWebhookConfiguration` in the Tenant Cluster:

```
$: kubectl get validatingwebhookconfigurations
NAME                       WEBHOOKS   AGE
kamaji-write-permissions   2          59m
```

The webhook intercepts all API requests to the Tenant Control Plane and programmatically denies any attempts to modify resources.

As a result, all changes initiated by tenants (such as `kubectl apply`, `kubectl delete`, or CRD updates) could be blocked.

!!! warning "Operators and Controller"
    When the write status is limited, all actions are intercepted by the webhook.
    If a Pod must be rescheduled, the webhook will deny it.

## Behaviour with limited write operations

If a tenant user tries to perform non-allowed write operations, such as:

- creating resources when `TenantControlPlane.spec.writePermissions.blockCreate` is set to `true`
- updating resources when `TenantControlPlane.spec.writePermissions.blockUpdate` is set to `true`
- deleting resources when `TenantControlPlane.spec.writePermissions.blockDelete` is set to `true`

the following error is returned:

```
Error from server (Forbidden): admission webhook "catchall.write-permissions.kamaji.clastix.io" denied the request:
the current Control Plane has limited write permissions, current changes are blocked:
removing the webhook may lead to an inconsistent state upon its completion
```

This guarantees the cluster remains in a frozen, consistent state, preventing partial updates or drift.

## Use Cases

Typical scenarios where ReadOnly mode is useful:

- **Planned Maintenance**: freeze workloads before performing upgrades or infrastructure changes.
- **Disaster Recovery**: lock the Tenant Control Plane to prevent accidental modifications during incident handling.
- **Auditing & Compliance**: ensure workloads cannot be altered during a compliance check or certification process.
- **Quota Enforcement**: preventing Datastore quote over commit in terms of storage size.

!!! info "Migrating the DataStore"
    In a similar manner, when migrating a Tenant Control Plane to a different store, similar enforcement is put in place.
    This is managed automatically by Kamaji: there's no need to toggle on and off the ReadOnly mode.
