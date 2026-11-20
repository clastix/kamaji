# Public API Server Address

Kamaji's default behavior is to use the IP address assigned to the Tenant Control Plane's Service when exposing the control plane endpoint. However, in many scenarios you may want to use a custom DNS hostname instead of an IP address. This is particularly useful when:

- You want to use a hostname that matches your organization's naming conventions
- You're using a DNS name that is already registered in your certificate authority's Subject Alternative Names (SANs)
- You want to avoid hardcoding IP addresses in kubeconfigs and cluster configurations

## Using a Public API Server Address

To use a custom hostname for your Tenant Control Plane's API server, specify the `publicAPIServerAddress` field in the `controlPlane.service` section of your TenantControlPlane spec:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: my-tenant
  namespace: kamaji-system
spec:
  controlPlane:
    service:
      serviceType: LoadBalancer
      publicAPIServerAddress: "my-tenant.k8s.example.com"
  kubernetes:
    version: v1.35.7
  networkProfile:
    port: 30000
```

## How It Works

When `publicAPIServerAddress` is specified:

1. **Control Plane Endpoint**: The value is used in the `status.controlPlaneEndpoint` field instead of the IP address
2. **Kubeconfigs**: The generated admin, controller-manager, and scheduler kubeconfigs will use this hostname as the server URL
3. **Certificate SANs**: The hostname is automatically added to the API server certificate's Subject Alternative Names (SANs)

This ensures that when clients connect using the kubeconfig, the certificate validation succeeds because the hostname matches the certificate SANs.

## Example Usage with cert-manager

When using cert-manager for certificate management, you can leverage this feature to ensure proper certificate validation:

1. Create a cert-manager Certificate that includes the desired hostname in its SANs:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: my-tenant-api-server
  namespace: kamaji-system
spec:
  secretName: my-tenant-api-server-cert
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
    - my-tenant.k8s.example.com
```

2. Configure your TenantControlPlane to use the hostname:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: my-tenant
  namespace: kamaji-system
spec:
  controlPlane:
    service:
      serviceType: LoadBalancer
      publicAPIServerAddress: "my-tenant.k8s.example.com"
  # ... rest of spec
```

## Requirements

- The hostname you specify must be resolvable via DNS to your Tenant Control Plane's API server
- If using a LoadBalancer service type, ensure your cloud provider/infrastructure can route traffic to the API server based on the hostname
- The hostname should be included in your API server certificate SANs if you're using custom certificates

## Benefits

- **Consistent naming**: Use hostnames that follow your organization's naming conventions
- **Certificate validation**: Avoid x509 certificate errors when connecting with kubeconfigs
- **Flexibility**: Decouple the API server address from the underlying IP address
- **Migration**: Easily migrate to new IP addresses without changing the hostname
