# Talos Linux worker nodes

[Talos Linux](https://www.talos.dev) nodes can join a Tenant Control Plane as
workers. The kubelet side works like any kubeadm join: a bootstrap token and
the tenant's CA. Talos needs two more things that a Talos control plane would
normally provide:

- **A secrets bundle built from the tenant's PKI**, so the worker trusts the
  Tenant Control Plane's API server.
- **trustd on port 50001 of the control plane endpoint.** A Talos worker's API
  service (`apid`) gets its certificate from trustd, which runs on Talos
  control plane nodes. A Tenant Control Plane has none, so `apid` never starts,
  the boot sequence fails at Talos's 70-minute boot timeout, and the worker
  reboots. The node joins fine, then reboots every 70 minutes.

This guide grafts the tenant's PKI into a Talos secrets bundle and runs
[kamaji-trustd](https://github.com/lachgar-Azzeddine/kamaji-trustd), a small
implementation of the trustd call workers make, as an additional container of
the Tenant Control Plane.

Tested with Talos v1.14 and Kubernetes v1.36.

## 1. Build a Talos secrets bundle from the tenant's PKI

Export the tenant's certificates in kubeadm layout. Talos also requires an etcd
CA in the bundle; workers never use it, so a throwaway one is enough.

```bash
NS=tenant-00
TCP=tenant-00
mkdir -p pki/etcd

kubectl -n $NS get secret $TCP-ca -o jsonpath='{.data.ca\.crt}' | base64 -d > pki/ca.crt
kubectl -n $NS get secret $TCP-ca -o jsonpath='{.data.ca\.key}' | base64 -d > pki/ca.key
kubectl -n $NS get secret $TCP-sa-certificate -o jsonpath='{.data.sa\.key}' | base64 -d > pki/sa.key
kubectl -n $NS get secret $TCP-sa-certificate -o jsonpath='{.data.sa\.pub}' | base64 -d > pki/sa.pub
kubectl -n $NS get secret $TCP-front-proxy-ca-certificate -o jsonpath='{.data.front-proxy-ca\.crt}' | base64 -d > pki/front-proxy-ca.crt
kubectl -n $NS get secret $TCP-front-proxy-ca-certificate -o jsonpath='{.data.front-proxy-ca\.key}' | base64 -d > pki/front-proxy-ca.key

openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 3650 \
  -subj "/CN=unused-etcd-ca" -keyout pki/etcd/ca.key -out pki/etcd/ca.crt
```

Create a bootstrap token in the tenant cluster, as `kubeadm token create` would:

```bash
kubeadm token create --kubeconfig tenant-00.kubeconfig --ttl 1h
# e.g. abcdef.0123456789abcdef
```

Then generate the bundle and keep it: every worker of this tenant shares its
OS CA and machine token.

```bash
talosctl gen secrets --from-kubernetes-pki pki \
  --kubernetes-bootstrap-token abcdef.0123456789abcdef -o secrets.yaml
```

## 2. Run trustd in the Tenant Control Plane

Store the bundle's OS CA and machine token in a Secret next to the Tenant
Control Plane:

```bash
yq '.certs.os.crt' secrets.yaml | base64 -d > ca.crt
yq '.certs.os.key' secrets.yaml | base64 -d > ca.key
yq '.trustdinfo.token' secrets.yaml > token
kubectl -n $NS create secret generic $TCP-talos-trustd \
  --from-file=ca.crt --from-file=ca.key --from-file=token
```

Add the container and expose port 50001 on the control plane Service:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: tenant-00
  namespace: tenant-00
spec:
  controlPlane:
    deployment:
      additionalContainers:
        - name: talos-trustd
          image: ghcr.io/lachgar-azzeddine/kamaji-trustd:main  # pin by digest in production
          args: ["--san=<control plane endpoint IP>"]
          ports:
            - name: talos-trustd
              containerPort: 50001
          volumeMounts:
            - name: talos-trustd
              mountPath: /var/run/kamaji-trustd
              readOnly: true
          securityContext:
            allowPrivilegeEscalation: false
            runAsNonRoot: true
            capabilities:
              drop: ["ALL"]
      additionalVolumes:
        - name: talos-trustd
          secret:
            secretName: tenant-00-talos-trustd
    service:
      additionalPorts:
        - name: talos-trustd
          port: 50001
          targetPort: 50001
          protocol: TCP
  # ...
```

kamaji-trustd behaves like Talos's own trustd: requests must carry the machine
token, only server certificates are issued, and any requested Talos role is
dropped, so a worker can never get a Talos client certificate.

## 3. Let nodes read EndpointSlices

Talos workers find trustd through the API server's EndpointSlices. A Talos
control plane grants nodes this access; in the tenant cluster, add it:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:talos-nodes
rules:
  - apiGroups: ["discovery.k8s.io"]
    resources: ["endpointslices"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:talos-nodes
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:talos-nodes
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: Group
    name: system:nodes
```

## 4. Generate the worker config and boot

Use the Tenant Control Plane's endpoint and Kubernetes version:

```bash
ENDPOINT=$(kubectl -n $NS get tcp $TCP -o jsonpath='{.status.controlPlaneEndpoint}')
VERSION=$(kubectl -n $NS get tcp $TCP -o jsonpath='{.spec.kubernetes.version}')

talosctl gen config $TCP https://$ENDPOINT --with-secrets secrets.yaml \
  --kubernetes-version ${VERSION#v} --output-types worker,talosconfig
```

The worker config holds no CA private keys. Boot the node with it as usual.
The kubelet joins with the bootstrap token, and the boot sequence completes
in seconds instead of timing out:

```bash
talosctl --talosconfig talosconfig -n <worker IP> -e <worker IP> service apid
# STATE Running, HEALTH OK
```

With `apid` running, the worker can be managed and upgraded with `talosctl`
like any Talos node.

## Security

The `<tcp>-talos-trustd` Secret holds the Talos OS CA key: whoever reads it can
mint Talos client certificates for the tenant's workers. Restrict it to the
people who own those workers.
