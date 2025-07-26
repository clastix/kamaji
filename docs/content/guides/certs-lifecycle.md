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
reporting the rotation date time as the annotation `certs.kamaji.clastix.io/rotate` value.

```
$: kubectl annotate secret -l kamaji.clastix.io/certificate_lifecycle_controller=x509 certs.kamaji.clastix.io/rotate=""
secret/k8s-133-api-server-certificate annotated
secret/k8s-133-api-server-kubelet-client-certificate annotated
secret/k8s-133-datastore-certificate annotated
secret/k8s-133-front-proxy-client-certificate annotated
secret/k8s-133-konnectivity-certificate annotated

$: kubectl get secrets -l kamaji.clastix.io/certificate_lifecycle_controller=x509 -ojson | jq -r '.items[] | "\(.metadata.name) rotated at \(.metadata.annotations["certs.kamaji.clastix.io/rotate"])"'
k8s-133-api-server-certificate rotated at 2025-07-15 15:15:08.842191367 +0200 CEST m=+325.785000014
k8s-133-api-server-kubelet-client-certificate rotated at 2025-07-15 15:15:10.468139865 +0200 CEST m=+327.410948506
k8s-133-datastore-certificate rotated at 2025-07-15 15:15:15.454468752 +0200 CEST m=+332.397277417
k8s-133-front-proxy-client-certificate rotated at 2025-07-15 15:15:13.279920467 +0200 CEST m=+330.222729097
k8s-133-konnectivity-certificate rotated at 2025-07-15 15:15:17.361431671 +0200 CEST m=+334.304240277
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
