# Gateway API

Kamaji provides built-in support for the [Gateway API](https://gateway-api.sigs.k8s.io/), allowing Tenant Control Planes to be exposed as SNI-based addresses/urls. This eliminates the need for a dedicated LoadBalancer IP per TCP. A single Gateway resource can be used for multiple Tenant Control Planes and provide access to them with hostname-based routing (like `https://mycluster.xyz.com:6443`).

You can configure Gateway in Tenant Control Plane via `tcp.spec.controlPlane.gateway`, Kamaji will automatically create a `TLSRoute` resource with corresponding spec. To make this configuration work, you need to ensure `gateway` exists (is created by you) and `tcp.spec.controlPlane.gateway` points to your gateway.

We will cover a few examples below on how this is done.


## Prerequisites

Before using Gateway API mode, please ensure:

1. **Gateway API CRDs are installed** in your cluster (Required CRDs: `GatewayClass`, `Gateway`, `TLSRoute`)

2. **A Gateway resource exists** with appropriate configuration (see examples in this guide):
    - Listeners for kube-apiserver.
    - Use TLS protocol with Passthrough mode
    - Hostname (or Hostname pattern) matching your Tenant Control Plane hostname

3. (optional) **DNS is configured** to resolve hostnames (or hostname pattern) to the Gateway's LoadBalancer IP address. (This is needed for worker nodes to join, for testing we will use host entries in `/etc/hosts` for this guide)

4. **Gateway controller is running** (e.g., Envoy Gateway, Istio Gateway, etc.)

To replicate the guide below, please install [Envoy Gateway](https://gateway.envoyproxy.io/docs/tasks/quickstart/).

Next, create a gateway resource:

#### Gateway Resource Setup

Your Gateway resource must have listeners configured for the control plane. Here's an example Gateway configuration:

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
```
The above gateway is configured with envoy gateway controller. You can achieve the same results with any other gateway controller that supports `TLSRoutes` and TLS passthrough mode.

The rest of this guide focuses on TCP. 

## TenantControlPlane Gateway Configuration

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
        sectionName: kube-apiserver
        port: 6443
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
    - "tcp1.cluster.dev"  # make sure to set this.
  addons:
    coreDNS: {}
    kubeProxy: {}

```

**Required fields:**

- `hostname`: The hostname that will be used for routing (must match Gateway listener hostname pattern)
- `parentRefs`: Array of Gateway references

**Optional fields:**

- `additionalMetadata.labels`: Custom labels to add to TLSRoute resources
- `additionalMetadata.annotations`: Custom annotations to add to TLSRoute resources

!!! info
    ### Verify

    From our kubectl client machine / remote machines we can access the cluster above with the hostname.
    
    **Step 1:** Fetch load balancer IP of the gateway.

    `kubectl get gateway gateway -n default -o jsonpath='{.status.addresses[0].value}'`

    **Step 2:** Add host entries in `/etc/hosts` with the above hostname and gateway LB IP.

    `echo "<LB_IP> tcp1.cluster.dev" | sudo tee -a /etc/hosts`

    **Step 3:** Fetch kubeconfig of tcp cluster.

    `kubectl get secrets  tcp-1-admin-kubeconfig -o jsonpath='{.data.admin\.conf}' | base64 -d > kubeconfig`

    **Step 4:** Test connectivity:
    
    `kubectl --kubeconfig kubeconfig cluster-info`



### Multiple Tenant Control Planes

We can use the same Gateway resource for multiple Tenant Control Planes by using different hostnames per tenant cluster.
Let us extend the above example for multiple tenant control planes behind single gateway (and LB IP).

```yaml
# Gateway with wildcard hostname
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: gateway
spec:
  listeners:
  - name: kube-apiserver
    port: 6443
    # note: we changed to wildcard hostname pattern matching
    # for cluster.dev
    hostname: '*.cluster.dev'
  
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

Each Tenant Control Plane needs to use a different hostname. For each TCP, Kamaji creates a `TLSRoutes` resource with the respective hostnames, all `TLSRoutes` routing through the same Gateway resource.


### Konnectivity

If konnectivity addon is enabled, Kamaji creates a separate TLSRoute for it. But this is hardcoded with the listener name `konnectivity-server` and port `8132`. All gateways mentioned in `spec.controlPlane.gateway.parentRefs` must contain a listener with the same configuration for the given hostname. Below is example configuration:
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: gateway
  namespace: default
spec:
  gatewayClassName: envoy-gw-class
  listeners:
  - ...
  - ...
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

## Additional Resources

- [Gateway API Documentation](https://gateway-api.sigs.k8s.io/)
- [Quickstart with Envoy Gateway](https://gateway.envoyproxy.io/docs/tasks/quickstart/)


