# Ingress Addon

A Kubernetes API Server could be announced to users in several ways.
The most preferred way is leveraging on Load Balancers with their dedicated IP.

![Load Balancer setup](../images/kamaji-addon-ingress-lb.png#only-light)
![Load Balancer setup](../images/kamaji-addon-ingress-lb-dark.png#only-dark)

However, IPv4 addresses could be limited and scarce in availability, as well as expensive for public ones when running in the Cloud.
A possible optimisation could be implementing an Ingress Controller which routes traffic to Kubernetes API Servers on a host-routing basis.

Despite this solution sounding optimal for end users, it brings some challenges from the worker nodes' standpoint.

## Challenges

Internally deployed applications that need to interact with the Kubernetes API Server will leverage on the `kubernetes` endpoint in the `default` namespace:
every request sent to the `https://kubernetes.default.svc` endpoint will be forwarded to the Kubernetes API Server.

The routing put in place by the Kubernetes CNI is based on the L4, meaning that all the requests will be forwarded to the Ingress Controller with no `Host` header,
making impossible a routing based on the FQDN.

## Solution

The `kamaji-addon-ingress` is an addon that will expose the Tenant Control Plane behind an Ingress Controller.
It's responsible for creating an `Ingress` object with the required HTTP rules, as well as the annotations needed for the TLS/SSL passthrough.

![Ingress Controller setup](../images/kamaji-addon-ingress-ic.png#only-light)
![Ingress Controller setup](../images/kamaji-addon-ingress-ic-dark.png#only-dark)

Following is the list of supported Ingress Controllers:

- [HAProxy Technologies Kubernetes Ingress](https://github.com/haproxytech/kubernetes-ingress)

!!! info "Other Ingress Controllers"
    Active subscribers can request support for additional Ingress Controller flavours.  

## How to enable the Addon

Annotate the Tenant Control Plane instances with the key `kamaji.clastix.io/ingress.domain` and the domain suffix domain value:

```shell
kubectl annotate tenantcontrolplane $NAME kamaji.clastix.io/ingress.domain=$SUFFIX_DOMAIN
```

The value must be the expected suffix domain of generated resources.

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  annotations:
    kamaji.clastix.io/ingress.domain: clastix.cloud  # the expected kamaji-addon-ingress label
  name: tenant-00
  namespace: apezzuto
```

Once a Tenant Control Plane has been annotated with this key, the addon will generate the following `Ingress` object.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    haproxy.org/ssl-passthrough: "true"
  name: 592ee7b8-cd07-48cf-b754-76f370c3f87c
  namespace: apezzuto
spec:
  ingressClassName: haproxy
  rules:
  - host: apezzuto-tenant-00.k8s.clastix.cloud
    http:
      paths:
      - backend:
          service:
            name: tenant-00
            port:
              number: 6443
        path: /
        pathType: Prefix
  - host: apezzuto-tenant-00.konnectivity.clastix.cloud
    http:
      paths:
      - backend:
          service:
            name: tenant-00
            port:
              number: 8132
        path: /
        pathType: Prefix
```

The pattern for the generated hosts is the following:
`${tcp.namespace}-${tcp.name}.{k8s|konnectivity}.${ADDON_ANNOTATION_VALUE}`. Please, notice the `konnectivity` rule will be created only if the `konnectivity` addon has been enabled.

## Infrastructure requirements

For Tenant Control Plane objects leveraging on this addon, the following changes must be implemented.

### Ingress Controller

The Ingress Controller must be deployed to listen for `https` connection on the default port `443`:
if you have different requirements, please, engage with the CLASTIX team.

### DNS resolution

The following zones must be configured properly according to your DNS provider:

```
*.konnectivity.clastix.cloud    A   <YOUR_INGRESS_CONTROLLER_IP>
*.k8s.clastix.cloud             A   <YOUR_INGRESS_CONTROLLER_IP>
```

### Certificate SANs

```yaml
  networkProfile:
    certSANs:
    - apezzuto-tenant-00.k8s.clastix.cloud
    - apezzuto-tenant-00.konnectivity.clastix.cloud
    dnsServiceIPs:
    - 10.96.0.10
    podCidr: 10.244.0.0/16
    port: 6443
    serviceCidr: 10.96.0.0/16
```

### Service type and Ingress

The Kubernetes API Server can be exposed using a `ClusterIP`, rather than a Load Balancer.

```yaml
spec:
  controlPlane:
    service:
      serviceType: ClusterIP
    ingress:
      hostname: apezzuto-tenant-00.k8s.clastix.cloud:443
      ingressClassName: unhandled
```

The `ingressClassName` value must match a non-handled `IngressClass` object,
the addon will take care of generating the correct object.

!!! warning "Use the right port"
    The `hostname` field must absolutely point to the 443 port!

### Kubernetes components extra Arguments

The Kubernetes API Server must start with the following flag:

```yaml
spec:
  controlPlane:
    deployment:
      extraArgs:
        apiServer:
          - --endpoint-reconciler-type=none
```

The `kamaji-addon-ingress` will be responsible for populating the `kubernetes` EndpointSlice object in the Tenant cluster.

If you're running with `konnectivity`, also this extra argument must be enforced:

```yaml
spec:
  addons:
    konnectivity:
      agent:
        extraArgs:
        - --proxy-server-host=apezzuto-tenant-00.konnectivity.clastix.cloud
        - --proxy-server-port=443
```

## Air-gapped environments

The `kamaji-addon-ingress` works with a deployed component in the Tenant Cluster based on the container image `docker.io/clastix/tcp-proxy:latest`.

The same image can be replaced by customising the Addon Helm value upon installation:

```
--set options.tcpProxyImage=private.repository.tld/tcp-proxy:latest
```
