# Pre-generated certificates

By default, Kamaji generates every certificate and key the Tenant Control Plane
needs: the Kubernetes CA, the API Server certificate, the kubelet client
certificate, the front proxy CA and client, and the service account key pair.

Some environments already manage these with their own PKI, for example
cert-manager issuers, Vault, or an existing cluster's CA. The
`preGeneratedCertificates` field lets you point Kamaji at existing Secrets
instead. Kamaji then uses those certificates and keys rather than generating new
ones.

Every field is optional. Set only the certificates you want to supply and let
Kamaji generate the rest.

## How it works

`spec.preGeneratedCertificates` references Secrets in the same namespace as the
TenantControlPlane. Each reference is validated by the admission webhook when
the TenantControlPlane is created or updated:

- The Secret must exist in the TenantControlPlane namespace.
- The certificate and private key must be present under the configured keys,
  and they must form a valid pair.
- For the service account key pair, the public key must be an RSA key in PEM
  PKIX format, and both keys must parse.

| Field | Secret keys (defaults) | Use it for |
|---|---|---|
| `ca` | `tls.crt`, `tls.key` | The Kubernetes CA that signs the other certificates |
| `apiServer` | `tls.crt`, `tls.key` | The kube-apiserver serving certificate |
| `kubeletClient` | `tls.crt`, `tls.key` | The client certificate the API Server uses to reach kubelets |
| `frontProxyCA` | `tls.crt`, `tls.key` | The front proxy CA |
| `frontProxyClient` | `tls.crt`, `tls.key` | The front proxy client certificate |
| `serviceAccount` | `sa.pub`, `sa.key` | The service account signing key pair |

Certificate references accept `certificateKey` and `privateKeyKey` to read
different keys from the Secret. Service account references accept
`publicKeyKey` and `privateKeyKey`.

## Constraints

- **Chain of trust.** When you supply `apiServer`, `kubeletClient`, or
  `frontProxyClient`, sign them with the CA you supply. If you omit `ca`, Kamaji
  generates its own CA, and a certificate you supply must be signed by that
  generated CA. Since you can't know that CA in advance, supply `ca` whenever
  you supply any leaf certificate.
- **No `certSANs`.** `preGeneratedCertificates` cannot be combined with
  `networkProfile.certSANs`. The API Server certificate you provide already
  carries its SANs, so put every name and IP clients use into that certificate.
- **Same namespace.** Cross-namespace Secret references are not supported.
- **Service account key type.** The public key must be RSA. The
  service account key is not used for TLS, so this does not conflict with the
  ECDSA recommendation below for certificates.

## Example with cert-manager

The example creates a self-signed root, a CA issued from it, and the leaf
certificates signed by that CA. It uses cert-manager's default Secret keys
(`tls.crt` and `tls.key`), so the references need no key overrides. Replace the
example names and addresses with your own.

```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned-root
  namespace: tenants
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: tcp-ca
  namespace: tenants
spec:
  isCA: true
  commonName: tcp-ca
  secretName: tcp-ca-cert
  privateKey:
    algorithm: ECDSA
    size: 256
  issuerRef:
    name: selfsigned-root
    kind: Issuer
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: tcp-ca
  namespace: tenants
spec:
  ca:
    secretName: tcp-ca-cert
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: tcp-apiserver
  namespace: tenants
spec:
  secretName: tcp-apiserver-cert
  commonName: kube-apiserver
  dnsNames:
    - tcp-example
    - tcp-example.tenants.svc
    - tcp-example.tenants.svc.cluster.local
    - kubernetes
    - kubernetes.default
    - kubernetes.default.svc
    - kubernetes.default.svc.cluster.local
    - api.tenant.example.com
  ipAddresses:
    - 203.0.113.10
  usages:
    - server auth
  privateKey:
    algorithm: ECDSA
    size: 256
  issuerRef:
    name: tcp-ca
    kind: Issuer
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: tcp-kubelet-client
  namespace: tenants
spec:
  secretName: tcp-kubelet-client-cert
  commonName: kube-apiserver-kubelet-client
  subject:
    organizations:
      - system:masters
  usages:
    - client auth
  privateKey:
    algorithm: ECDSA
    size: 256
  issuerRef:
    name: tcp-ca
    kind: Issuer
```

The `tcp-ca-cert` Secret, created by the root Certificate, is the CA that
Kamaji uses. The kubelet client certificate uses the same subject as kubeadm's
`kube-apiserver-kubelet-client` certificate: common name
`kube-apiserver-kubelet-client` in the `system:masters` group.

Create the service account key pair yourself. cert-manager does not issue it:

```bash
openssl genrsa -out sa.key 2048
openssl rsa -in sa.key -pubout -out sa.pub
kubectl --context mgmt -n tenants create secret generic tcp-sa-keys \
  --from-file=sa.key --from-file=sa.pub
```

Then reference the Secrets from the TenantControlPlane:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: tcp-example
  namespace: tenants
spec:
  controlPlane:
    deployment:
      replicas: 2
    service:
      serviceType: LoadBalancer
  kubernetes:
    version: v1.30.0
  networkProfile:
    port: 6443
    address: 203.0.113.10
  dataStore: default
  preGeneratedCertificates:
    ca:
      secretName: tcp-ca-cert
    apiServer:
      secretName: tcp-apiserver-cert
    kubeletClient:
      secretName: tcp-kubelet-client-cert
    serviceAccount:
      secretName: tcp-sa-keys
```

The example leaves `frontProxyCA` and `frontProxyClient` unset, so Kamaji
generates the front proxy CA and client certificate. To supply them, add
`frontProxyCA` and `frontProxyClient` the same way. Sign the client certificate
with the front proxy CA you supply, not with `ca`.

Apply the TenantControlPlane. If a Secret is missing, has the wrong keys, or a
certificate is not a valid pair, the webhook rejects the object and the error
names the offending reference.
