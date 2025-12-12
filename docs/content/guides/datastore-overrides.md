# Datastore Overrides

Kamaji offers the possibility of having multiple ETCD clusters backing different resources of the k8s api server by configuring the [`--etcd-servers-overrides`](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/#:~:text=%2D%2Detcd%2Dservers%2Doverrides%20strings) flag. This feature can be useful for massive clusters to store resources with high churn in a dedicated ETCD cluster.

## Install Datastores

Create a self-signed cert-manager `ClusterIssuer`.
```bash
echo 'apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: self-signed
spec:
  selfSigned: {}
' | kubectl apply -f -
```

Install two Datastores, a primary and a secondary that will be used for `/events` resources.
```bash
 helm install etcd-primary clastix/kamaji-etcd -n kamaji-etcd --create-namespace \
   --set selfSignedCertificates.enabled=false \
   --set certManager.enabled=true \
   --set certManager.issuerRef.kind=ClusterIssuer \
   --set certManager.issuerRef.name=self-signed
```

For the secondary Datastore, use the cert-manager CA created by the `etcd-primary` helm release.
```bash
 helm install etcd-secondary clastix/kamaji-etcd -n kamaji-etcd --create-namespace \
   --set selfSignedCertificates.enabled=false \
   --set certManager.enabled=true \
   --set certManager.ca.create=false \
   --set certManager.ca.nameOverride=etcd-primary-kamaji-etcd-ca \
   --set certManager.issuerRef.kind=ClusterIssuer \
   --set certManager.issuerRef.name=self-signed
```

## Create a Tenant Control Plane

Using the `spec.dataStoreOverrides` field, Datastores different from the one used in `spec.dataStore` can be used to store specific resources.

```bash
echo 'apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: k8s-133
  labels:
    tenant.clastix.io: k8s-133
spec:
  controlPlane:
    deployment:
      replicas: 2
    service:
      serviceType: LoadBalancer
  kubernetes:
    version: "v1.33.1"
    kubelet:
      cgroupfs: systemd
  dataStore: etcd-primary-kamaji-etcd
  dataStoreOverrides:
    - resource: "/events" # Store events in the secondary ETCD
      dataStore: etcd-secondary-kamaji-etcd
  networkProfile:
    port: 6443
  addons:
    coreDNS: {}
    kubeProxy: {}
    konnectivity:
      server:
        port: 8132
      agent:
        mode: DaemonSet
' | k apply -f -
```

## Considerations

Only built-in resources can be tagetted by `--etcd-servers-overrides`, it is currently not possible to target Custom Resources.
