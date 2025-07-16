# Pausing Reconciliations

Kamaji follows the Kubernetes Operator pattern, which includes implementing a reconciliation loop.
This loop continuously reacts to events such as creation, updates, and deletions of resources.

To temporarily disable reconciliation for a resource, you can use the following annotation:
> `kamaji.clastix.io/paused`

!!! info "Annotation value"
    The annotation key is sufficient on its own: no value is required.
    Its mere presence disables controller reconciliations.

## Pausing `TenantControlPlane` reconciliations

When you add the `kamaji.clastix.io/paused` annotation to a TenantControlPlane object,
Kamaji will halt all reconciliation processes for that object.

This affects **all controllers**, including:

- The primary controller responsible for provisioning resources in the management cluster
- Secondary (soot) controllers responsible for bootstrapping the control plane, deploying addons, and managing any additional resources handled by Kamaji.

## Pausing Secret rotation

Kamaji automatically generates and manages several `Secret` resources, such as:

- `x509` certificates
- `kubeconfig` credentials

These secrets are automatically rotated by Kamaji's built-in **Certificate Lifecycle** feature.

To temporarily disable secret rotation for these resources,
apply the `kamaji.clastix.io/paused` annotation to the corresponding object.
