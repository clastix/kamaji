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

If you need to manually rotate one of these certificates, the required operation is the deletion for the given Secret.

```
$: kubectl get secret
NAME                                            TYPE                DATA   AGE
k8s-126-admin-kubeconfig                        Opaque              1      12m
k8s-126-api-server-certificate                  Opaque              2      12m
k8s-126-api-server-kubelet-client-certificate   Opaque              2      3h45m
k8s-126-ca                                      Opaque              4      3h45m
k8s-126-controller-manager-kubeconfig           Opaque              1      3h45m
k8s-126-datastore-certificate                   Opaque              3      3h45m
k8s-126-datastore-config                        Opaque              4      3h45m
k8s-126-front-proxy-ca-certificate              Opaque              2      3h45m
k8s-126-front-proxy-client-certificate          Opaque              2      3h45m
k8s-126-konnectivity-certificate                kubernetes.io/tls   2      3h45m
k8s-126-konnectivity-kubeconfig                 Opaque              1      3h45m
k8s-126-sa-certificate                          Opaque              2      3h45m
k8s-126-scheduler-kubeconfig                    Opaque              1      3h45m
```

Once this operation is performed, Kamaji will be notified of the missing certificate, and it will create it back.

```
$: kubectl delete secret -l kamaji.clastix.io/certificate_lifecycle_controller=x509
secret "k8s-126-api-server-certificate" deleted
secret "k8s-126-api-server-kubelet-client-certificate" deleted
secret "k8s-126-front-proxy-client-certificate" deleted
secret "k8s-126-konnectivity-certificate" deleted

$: kubectl delete secret -l kamaji.clastix.io/certificate_lifecycle_controller=x509
NAME                                            TYPE                DATA   AGE
k8s-126-admin-kubeconfig                        Opaque              1      15m
k8s-126-api-server-certificate                  Opaque              2      12s
k8s-126-api-server-kubelet-client-certificate   Opaque              2      12s
k8s-126-ca                                      Opaque              4      3h48m
k8s-126-controller-manager-kubeconfig           Opaque              1      3h48m
k8s-126-datastore-certificate                   Opaque              3      3h48m
k8s-126-datastore-config                        Opaque              4      3h48m
k8s-126-front-proxy-ca-certificate              Opaque              2      3h48m
k8s-126-front-proxy-client-certificate          Opaque              2      12s
k8s-126-konnectivity-certificate                kubernetes.io/tls   2      11s
k8s-126-konnectivity-kubeconfig                 Opaque              1      3h48m
k8s-126-sa-certificate                          Opaque              2      3h48m
k8s-126-scheduler-kubeconfig                    Opaque              1      3h48m
```

You can notice the secrets have been automatically created back, as well as a TenantControlPlane rollout with the updated certificates.

```
$: kubectl get pods
NAME                       READY   STATUS    RESTARTS   AGE
k8s-126-76768bdf89-82w8g   4/4     Running   0          58s
k8s-126-76768bdf89-fwltl   4/4     Running   0          58s
```

The same occurs with the `kubeconfig` ones.

```
$: kubectl delete secret -l kamaji.clastix.io/certificate_lifecycle_controller=kubeconfig
secret "k8s-126-admin-kubeconfig" deleted
secret "k8s-126-controller-manager-kubeconfig" deleted
secret "k8s-126-konnectivity-kubeconfig" deleted
secret "k8s-126-scheduler-kubeconfig" deleted

$: kubectl get pods
NAME                       READY   STATUS    RESTARTS   AGE
k8s-126-576c775b5d-2gr9h   4/4     Running   0          50s
k8s-126-576c775b5d-jmvlm   4/4     Running   0          50s
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

This can be rotated manually by deleting the following secret.

```
$: kubectl delete secret k8s-126-ca
secret "k8s-126-ca" deleted
```

Once this occurs the TenantControlPlane will enter in the `CertificateAuthorityRotating` status.

```
$: kubectl get tcp -w
NAME      VERSION   STATUS   CONTROL-PLANE ENDPOINT   KUBECONFIG                 DATASTORE   AGE
k8s-126   v1.26.0   Ready    172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   CertificateAuthorityRotating   172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   Ready                          172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   Ready                          172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   Ready                          172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   Ready                          172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
k8s-126   v1.26.0   Ready                          172.18.255.200:6443      k8s-126-admin-kubeconfig   default     3h58m
```

This operation is intended to be performed manually since a new Certificate Authority requires the restart of all the components, as well as of the nodes:
in such case, you will need to distribute the new Certificate Authority and the new nodes certificates.

Given the sensibility of such operation, the `Secret` controller will not check the _CA_, which is offering validity of 10 years as `kubeadm` default values. 
