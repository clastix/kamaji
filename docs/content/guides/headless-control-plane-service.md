# Headless control plane Service

By default, Kamaji exposes the Tenant Control Plane through a Service with a
virtual IP (VIP): a `ClusterIP` allocated by the management cluster, a cloud
load balancer IP, or NodePorts depending on `serviceType`.

Some deployments don't need — or can't use — a Service VIP:

- **VPC-isolated CNIs** (e.g. Kube-OVN multi-VPC): a Service VIP is only
  reachable through the VPC whose load balancer it is attached to, so the
  ClusterIP of the control plane Service may be unusable from other VPCs
  (including the one hosting the Kamaji controller itself, see
  [clastix/kamaji#1384](https://github.com/clastix/kamaji/issues/1384)).
- Consumers doing their own DNS-based discovery against pod IPs avoid the
  kube-proxy/conntrack hop on the API server path.
- No Service VIP needs to be carved out of a CIDR that may overlap with pod
  or overlay ranges.

For these cases, the control plane Service can be deployed headless: DNS
resolves the Service name directly to the backing pod IPs.

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: headless-tcp
spec:
  controlPlane:
    service:
      serviceType: ClusterIP
      headless: true
  networkProfile:
    # Required with headless: a headless Service has no VIP to announce,
    # so the control plane endpoint must be declared explicitly.
    address: 10.0.0.10
    port: 6443
```

## Rules

- `headless` is only valid when `serviceType` is `ClusterIP`.
- `spec.networkProfile.address` must be set: a headless Service has no VIP to
  announce as the control plane endpoint (kubeconfig server URLs, kubeadm
  join, and `status.ControlPlaneEndpoint` all use the declared address).
- The field is immutable: it cannot be changed, set, or unset after creation,
  mirroring the immutability of the Kubernetes `clusterIP` field itself.

## Certificate SANs

The API server certificate automatically includes the declared
`networkProfile.address`. With a headless Service there is no ClusterIP to
add to the SANs, so clients must connect using the declared address (or
additional names listed in `spec.networkProfile.certSANs`).
