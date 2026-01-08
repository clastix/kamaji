# Gateway API Support

Kamaji provides built-in support for the [Gateway API](https://gateway-api.sigs.k8s.io/), allowing you to expose Tenant Control Planes using TLSRoute resources with SNI-based routing. This enables hostname-based routing to multiple Tenant Control Planes through a single Gateway resource, reducing the need for dedicated LoadBalancer services.

## Overview

Gateway API support in Kamaji automatically creates and manages TLSRoute resources for your Tenant Control Planes. When you configure a Gateway for a Tenant Control Plane, Kamaji automatically creates TLSRoutes for the Control Plane API Server. If konnectivity is enabled, a separate TLSRoute is created for it. Both TLSRoutes use the same hostname and Gateway resource, but route to different ports(listeners) using port-based routing and semantic `sectionName` values.

Therefore, the target `Gateway` resource must have right listener configurations (see the Gateway [example section](#gateway-resource-setup) below).


## How It Works

When you configure `spec.controlPlane.gateway` in a TenantControlPlane resource, Kamaji automatically:

1. **Creates a TLSRoute for the control plane** that routes for port 6443 (or `spec.networkProfile.port`) with sectionName `"kube-apiserver"`
2. **Creates a TLSRoute for Konnectivity** (if konnectivity addon is enabled) that routes for port 8132 (or `spec.addons.konnectivity.server.port`) with sectionName `"konnectivity-server"`

Both TLSRoutes:

- Use the same hostname from `spec.controlPlane.gateway.hostname`
- Reference the same parent Gateway resource via `parentRefs`
- The `port` and `sectionName` fields are set automatically by Kamaji
- Route to the appropriate Tenant Control Plane service

The Gateway resource must have listeners configured for both ports (6443 and 8132) to support both routes.

## Prerequisites

Before using Gateway API support, ensure:

1. **Gateway API CRDs are installed** in your cluster (Required CRDs: `GatewayClass`, `Gateway`, `TLSRoute`)

2. **A Gateway resource exists** with appropriate listeners configured:
    - At minimum, listeners for ports 6443 (control plane) and 8132 (Konnectivity)
    - TLS protocol with Passthrough mode
    - Hostname pattern matching your Tenant Control Plane hostnames

3. **DNS is configured** to resolve your hostnames to the Gateway's external address

4. **Gateway controller is running** (e.g., Envoy Gateway, Istio Gateway, etc.)

## Configuration

### TenantControlPlane Gateway Configuration

Enable Gateway API mode by setting the `spec.controlPlane.gateway` field in your TenantControlPlane resource:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: tcp-1
spec:
  controlPlane:
  # ... gateway configuration:
    gateway:
      hostname: "tcp1.cluster.dev"
      parentRefs:
      - name: gateway
        namespace: default
      additionalMetadata:
        labels:
          environment: production
        annotations:
          example.com/custom: "value"
  # ... rest of the spec
    deployment:
      replicas: 1
    service:
      serviceType: ClusterIP
  dataStore: default
  kubernetes:
    version: v1.29.0
    kubelet:
      cgroupfs: systemd
  networkProfile:
    port: 6443
    certSANs:
    - "c11.cluster.dev"  # make sure to set this.
  addons:
    coreDNS: {}
    kubeProxy: {}
    konnectivity: {}

```

**Required fields:**

- `hostname`: The hostname that will be used for routing (must match Gateway listener hostname pattern)
- `parentRefs`: Array of Gateway references (name and namespace)

**Optional fields:**

- `additionalMetadata.labels`: Custom labels to add to TLSRoute resources
- `additionalMetadata.annotations`: Custom annotations to add to TLSRoute resources

!!! warning "Port and sectionName are set automatically"
    Do not specify `port` or `sectionName` in `parentRefs`. Kamaji automatically sets these fields in TLSRoutes.

### Gateway Resource Setup

Your Gateway resource must have listeners configured for both the control plane and Konnectivity ports. Here's an example Gateway configuration:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: envoy-gw-class
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: gateway
  namespace: default
spec:
  gatewayClassName: envoy-gw-class
  listeners:
  - name: kube-apiserver
    port: 6443
    protocol: TLS
    hostname: 'tcp1.cluster.dev'
    tls:
      mode: Passthrough
    allowedRoutes:
      kinds:
      - group: gateway.networking.k8s.io
        kind: TLSRoute
      namespaces:
        from: All

    # if konnectivity addon is enabled:
  - name: konnectivity-server
    port: 8132
    protocol: TLS
    hostname: 'tcp1.cluster.dev'
    tls:
      mode: Passthrough
    allowedRoutes:
      kinds:
      - group: gateway.networking.k8s.io
        kind: TLSRoute
      namespaces:
        from: All
    
```


## Multiple Tenant Control Planes

You can use the same Gateway resource for multiple Tenant Control Planes by using different hostnames:

```yaml
# Gateway with wildcard hostname
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: gateway
spec:
  listeners:
  - hostname: '*.cluster.dev'
    name: kube-apiserver
    port: 6443
    # ...
---
# Tenant Control Plane 1
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: tcp-1
spec:
  controlPlane:
    gateway:
      hostname: "tcp1.cluster.dev"
      parentRefs:
      - name: gateway
        namespace: default
  # ...
---
# Tenant Control Plane 2
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: tcp-2
spec:
  controlPlane:
    gateway:
      hostname: "tcp2.cluster.dev"
      parentRefs:
      - name: gateway
        namespace: default
  # ...
```

Each Tenant Control Plane will get its own TLSRoutes with the respective hostnames, all routing through the same Gateway resource.

You can check the Gateway status in the TenantControlPlane:

```bash
kubectl get tenantcontrolplane tcp-1 -o yaml
```

Look for the `status.kubernetesResources.gateway` and `status.addons.konnectivity.gateway` fields.


## Additional Resources

- [Gateway API Documentation](https://gateway-api.sigs.k8s.io/)
- [Quickstart with Envoy Gateway](https://gateway.envoyproxy.io/docs/tasks/quickstart/)


