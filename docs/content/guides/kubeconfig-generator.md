# Kubeconfig Generator

The **Kubeconfig Generator** is a Kamaji extension that simplifies the distribution of Kubeconfig files for tenant clusters managed through Kamaji.

Instead of manually exporting and editing credentials, the generator automates the creation of kubeconfigs aligned with your organizational policies.

## Motivation

When managing multiple Tenant Control Planes (TCPs), cluster administrators often face two challenges:

1. **Consistency**: ensuring kubeconfigs are generated with the correct user identity, groups, and endpoints
2. **Scalability**: distributing kubeconfigs to users across potentially dozens of tenant clusters without manual steps

The `KubeconfigGenerator` resource addresses these problems by:

- Selecting which TCPs to target via label selectors.
- Defining how to build user and group identities in kubeconfigs.
- Automatically maintaining kubeconfigs as new tenant clusters are created or updated.

This provides a single, declarative way to manage kubeconfig lifecycle across all your tenants,
especially convenient if those cases where an Identity Provider can't be used to delegate access to Tenant Control Plane clusters.

## How it Works

### Selection

- `namespaceSelector` filters the namespaces from which Tenant Control Planes are discovered.
- `tenantControlPlaneSelector` further refines which TCPs to include.

### Identity Definition

The `user` and `groups` fields use compound values, which can be either:

- A static string (e.g., `developer`)
- A dynamic reference resolved from the TCP object (e.g., `metadata.name`)

This allows kubeconfigs to be tailored to the cluster’s context or a fixed organizational pattern.

### Endpoint Resolution

The generator pulls the API server endpoint from the TCP’s `admin` kubeconfig.

By default it uses the `admin.svc` template, but this can be overridden with the `controlPlaneEndpointFrom` field.

### Status and Errors

The resource keeps track of how many kubeconfigs were attempted, how many succeeded,
and provides detailed error reports for failed generations.

## Typical Use Cases

- **Platform Operators**: automatically distribute kubeconfigs to developers as new tenant clusters are provisioned.
- **Multi-team Environments**: ensure each team gets kubeconfigs with the correct groups for RBAC authorization.
- **Least Privilege Principle**: avoid distributing `cluster-admin` credentials with a fine-grained RBAC
- **Dynamic Access**: use `fromDefinition` references to bind kubeconfig identities directly to tenant metadata
 (e.g., prefixing users with the TCP's name).

## Example Scenario

A SaaS provider runs multiple Tenant Control Planes, each corresponding to a different customer.
Instead of manually managing kubeconfigs for every customer environment, the operator defines a single `KubeconfigGenerator`:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: KubeconfigGenerator
metadata:
  name: tenant
spec:
  # Select only Tenant Control Planes living in namespaces
  # labeled as production environments
  namespaceSelector:
    matchLabels:
      environment: production
  # Match all Tenant Control Planes in those namespaces
  tenantControlPlaneSelector: {}
  # Assign a static group "customer-admins"
  groups:
    - stringValue: "customer-admins"
  # Derive the user identity dynamically from the TenantControlPlane metadata
  user:
    fromDefinition: "metadata.name"
  # Use the public admin endpoint from the TCP’s kubeconfig
  controlPlaneEndpointFrom: "admin.conf"
```

- Matches all TCPs in namespaces labeled `environment=production`.
- Generates kubeconfigs with group `customer-admins`.
- Derives the user identity from the TCP’s `metadata.name`.

As new tenants are created, their kubeconfigs are generated automatically and kept up to date.

```
$: kubectl get secret --all-namespaces -l kamaji.clastix.io/managed-by=tenant
NAMESPACE       NAME             TYPE     DATA   AGE
alpha-tnt       env-133-tenant   Opaque   1      12h
alpha-tnt       env-130-tenant   Opaque   1      2d
bravo-tnt       prod-tenant      Opaque   1      2h
charlie-tnt     stable-tenant    Opaque   1      1d
```

## Observability

The generator exposes its status directly in the CRD:
- `resources`: total number of TCPs targeted.
- `availableResources`: successfully generated kubeconfigs.
- `errors`: list of failed kubeconfig generations, including the affected resource and error message.

This allows quick debugging and operational awareness.

## Deployment

The _Kubeconfig Generator_ is **not** enabled by default since it's still in experimental state.

It can be enabled using the Helm value `kubeconfigGenerator.enabled=true` which is defaulted to `false`.
