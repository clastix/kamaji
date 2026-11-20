# Certificates Lifecycle

Kamaji is responsible for creating the required certificates, such as:

- the Kubernetes API Server certificate
- the Kubernetes API Server kubelet client certificate
- the Datastore certificate
- the front proxy client certificate
- the konnectivity certificate (if enabled)

Also, the following `kubeconfig` resources contain client certificates, which are created by Kamaji, such as:

- `admin`
- `controller-manager`
- `konnectivity` (if enabled)
- `scheduler`

All the certificates are created with the `kubeadm` defaults, thus their validity is set to 1 year.

## How to rotate certificates

All certificates can be rotated at the same time, or one by one: this is possible by annotating resources using
the well-known annotation `certs.kamaji.clastix.io/rotate`.

```
$: kubectl get secret
NAME                                            TYPE                DATA   AGE
k8s-133-admin-kubeconfig                        Opaque              1      12m
k8s-133-api-server-certificate                  Opaque              2      12m
k8s-133-api-server-kubelet-client-certificate   Opaque              2      3h45m
k8s-133-ca                                      Opaque              4      3h45m
k8s-133-controller-manager-kubeconfig           Opaque              1      3h45m
k8s-133-datastore-certificate                   Opaque              3      3h45m
k8s-133-datastore-config                        Opaque              4      3h45m
k8s-133-front-proxy-ca-certificate              Opaque              2      3h45m
k8s-133-front-proxy-client-certificate          Opaque              2      3h45m
k8s-133-konnectivity-certificate                kubernetes.io/tls   2      3h45m
k8s-133-konnectivity-kubeconfig                 Opaque              1      3h45m
k8s-133-sa-certificate                          Opaque              2      3h45m
k8s-133-scheduler-kubeconfig                    Opaque              1      3h45m
```

Once this operation is performed, Kamaji will trigger a certificate renewal,
reporting the rotation date time as the annotation `certs.kamaji.clastix.io/rotate` value in the [RFC3339](https://pkg.go.dev/time#RFC3339) format.

```
$: kubectl annotate secret -l kamaji.clastix.io/certificate_lifecycle_controller=x509 certs.kamaji.clastix.io/rotate=""
secret/k8s-133-api-server-certificate annotated
secret/k8s-133-api-server-kubelet-client-certificate annotated
secret/k8s-133-datastore-certificate annotated
secret/k8s-133-front-proxy-client-certificate annotated
secret/k8s-133-konnectivity-certificate annotated

$: kubectl get secrets -l kamaji.clastix.io/certificate_lifecycle_controller=x509 -ojson | jq -r '.items[] | "\(.metadata.name) rotated at \(.metadata.annotations["certs.kamaji.clastix.io/rotate"])"'
k8s-133-api-server-certificate rotated at 2025-07-15T15:15:08Z02:00
k8s-133-api-server-kubelet-client-certificate rotated at 2025-07-15T15:15:10Z0200
k8s-133-datastore-certificate rotated at 2025-07-15T15:15:15Z0200
k8s-133-front-proxy-client-certificate rotated at 2025-07-15T15:15:13Z0200
k8s-133-konnectivity-certificate rotated at 2025-07-15T15:15:17Z0200
```

You can notice the secrets have been automatically created back, as well as a TenantControlPlane rollout with the updated certificates.

```
$: kubectl get pods
NAME                       READY   STATUS    RESTARTS   AGE
k8s-133-67bf496c8c-27bmp   4/4     Running   0          4m52s
k8s-133-67bf496c8c-x4t76   4/4     Running   0          4m52s
```

The same occurs with the `kubeconfig` ones.

```
$: kubectl annotate secret -l kamaji.clastix.io/certificate_lifecycle_controller=kubeconfig certs.kamaji.clastix.io/rotate=""
secret/k8s-133-admin-kubeconfig annotated
secret/k8s-133-controller-manager-kubeconfig annotated
secret/k8s-133-konnectivity-kubeconfig annotated
secret/k8s-133-scheduler-kubeconfig annotated

$: kubectl get secrets -l kamaji.clastix.io/certificate_lifecycle_controller=kubeconfig -ojson | jq -r '.items[] | "\(.metadata.name) rotated at \(.metadata.annotations["certs.kamaji.clastix.io/rotate"])"'
k8s-133-admin-kubeconfig rotated at 2025-07-15 15:20:41.688181782 +0200 CEST m=+658.630990441
k8s-133-controller-manager-kubeconfig rotated at 2025-07-15 15:20:42.712211056 +0200 CEST m=+659.655019677
k8s-133-konnectivity-kubeconfig rotated at 2025-07-15 15:20:46.405567865 +0200 CEST m=+663.348376504
k8s-133-scheduler-kubeconfig rotated at 2025-07-15 15:20:46.333718563 +0200 CEST m=+663.276527216
```

## Automatic certificates rotation

The Kamaji operator will run a controller which processes all the Secrets to determine their expiration, both for the `kubeconfig`, as well as for the certificates.

The controller, named `CertificateLifecycle`, will extract the certificates from the _Secret_ objects notifying the `TenantControlPlaneReconciler` controller which will start a new certificate rotation.
By default, the rotation will occur the day before their expiration.

This rotation deadline can be dynamically configured using the Kamaji CLI flag `--certificate-expiration-deadline` using the Go _Duration_ syntax:
e.g.: set the value `7d` to trigger the renewal a week before the effective expiration date.

!!! info "Other Datastore Drivers"
    Kamaji is responsible for creating the `etcd` client certificate, and the generation of a new one will occur.
    
    For other Datastore drivers, such as MySQL, PostgreSQL, or NATS, the referenced Secret will always be deleted by the Controller to trigger the rotation: the PKI management, since it's offloaded externally, must provide the renewed certificates.

## Certificate Authority rotation

Kamaji is also taking care of your Tenant Clusters Certificate Authority.

This can be rotated manually like other certificates by using the annotation `certs.kamaji.clastix.io/rotate`

```
$: kubectl annotate secret k8s-133-ca certs.kamaji.clastix.io/rotate="" 
secret/k8s-133-ca annotated
```

Once this occurs the TenantControlPlane will enter in the `CertificateAuthorityRotating` status.

```
$: kubectl get tcp -w
NAME      VERSION   STATUS   CONTROL-PLANE ENDPOINT   KUBECONFIG                 DATASTORE   AGE
k8s-133   v1.33.0   Ready    172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   Ready                          172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   Ready                          172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   Ready                          172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   Ready                          172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
k8s-133   v1.33.0   Ready                          172.18.255.200:6443      k8s-133-admin-kubeconfig   default     3h58m
```

This operation is intended to be performed manually since a new Certificate Authority requires the restart of all the components,
as well as of the nodes: in such a case, you will need to distribute the new Certificate Authority and the new nodes certificates.

Given the sensibility of such operation, the `Secret` controller will not check the _CA_, which is offering validity of 10 years as `kubeadm` default values.

### Overlapping Certificate Authority rotation

A Certificate Authority can also be rotated without a disruption, by trusting both the old and the new one
while the certificates signed by the old one are replaced.

Generate the new CA with `kubeadm`, which produces the same certificate Kamaji generates (`CN=kubernetes`, 10 years of validity)
and a private key in the PEM encoding Kamaji expects, PKCS#1 (`RSA PRIVATE KEY`), SEC1 (`EC PRIVATE KEY`) or PKCS#8 (`PRIVATE KEY`):

```
$: kubeadm init phase certs ca --cert-dir ./new-pki
[certs] Generating "ca" certificate and key
```

!!! note
    A CA generated by other tools must be a self-signed certificate with the `CA:TRUE` basic constraint and the `Certificate Sign` key usage,
    and its private key must use one of the PEM encodings above: with OpenSSL 3, `openssl genrsa` writes PKCS#8, `openssl genrsa -traditional` writes PKCS#1.

The CA `Secret` must then be edited by hand, putting in `ca.crt` the new CA **first**, followed by the old one,
and in `ca.key` the private key of the new CA:

```
$: kubectl get secret k8s-133-ca -o jsonpath='{.data.ca\.crt}' | base64 -d > old-ca.crt
$: cat new-pki/ca.crt old-ca.crt > ca.crt
$: kubectl patch secret k8s-133-ca --type merge -p "{\"data\":{\"ca.crt\":\"$(base64 < ca.crt | tr -d '\n')\",\"ca.key\":\"$(base64 < new-pki/ca.key | tr -d '\n')\"}}"
```

Kamaji refuses the `Secret` and stops reconciling the `TenantControlPlane` when the first certificate doesn't match `ca.key`,
or when any certificate of the bundle is malformed. The annotation `certs.kamaji.clastix.io/rotate` is refused on a bundle as well,
since it would regenerate a single CA dropping both: if set by mistake, remove it **before** removing the old CA from the bundle,
otherwise the pending request is honoured as soon as the bundle holds a single CA, generating a third CA trusted by nobody.

```
$: kubectl annotate secret k8s-133-ca certs.kamaji.clastix.io/rotate-
```

Once the bundle is set:

- the kubeconfigs generated by Kamaji, including the ones of the `KubeconfigGenerator` resources, are regenerated right away:
  their client certificates are signed by the new CA, and the whole bundle is embedded as CA data;
- the x509 certificates signed by the old CA, such as the API Server one, are kept as long as the old CA is trusted,
  while the new ones are signed by the new CA;
- the `cluster-info` _ConfigMap_ embeds the whole bundle as well, so that joining nodes trust both CAs;
- `tls.crt` holds the signing CA only, as required by the Cluster API CA hash pinning: it must not be used as a trust bundle, use `ca.crt` instead;
- the control plane _Deployment_ is rolled out to mount the signing CA for the `kube-controller-manager` CSR signer:
  until the rollout completes, the running `kube-controller-manager` cannot sign CSRs, nodes will retry.

Then, **before** replacing any certificate signed by the old CA, distribute the CA bundle to every existing consumer of the Tenant Cluster:
Kamaji updates the kubeconfigs it generates and the `cluster-info` _ConfigMap_, but the nodes and clients already running keep trusting
the CA they were given, such as `/etc/kubernetes/pki/ca.crt` and the kubelet kubeconfig on the nodes, or the kubeconfigs handed out to users
and tools. Reload them and verify every node and client trusts both CAs.

Only then, rotate the certificates signed by the old CA, such as the API Server one, with the annotation `certs.kamaji.clastix.io/rotate`:
a consumer still trusting the old CA alone would reject the certificate signed by the new one.

When the old CA is no longer required, remove it from `ca.crt`: the certificates and kubeconfigs still signed by it are reissued,
the single CA behaviour is restored, and the old CA can be removed from the nodes and clients. 
