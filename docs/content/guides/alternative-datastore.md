# Alternative Datastores

Kamaji offers the possibility of having a different storage system than `etcd` thanks to [kine](https://github.com/k3s-io/kine) integration.

## Installing Drivers

The following `make` recipes help you to setup alternative `Datastore` resources. On the Management Cluster, you can use the following commands:

- **MySQL**: `$ make -C deploy/kine/mysql mariadb`

- **PostgreSQL**: `$ make -C deploy/kine/postgresql postgresql`

- **NATS**: `$ make -C deploy/kine/nats nats`

!!! warning "Not for production"
    The default settings are not production grade: the following scripts are just used to test the Kamaji usage of different drivers.

## Defining a default Datastore upon Kamaji installation

Use Helm to install the Kamaji Operator, making sure it uses a datastore with the proper driver `datastore.driver=<MySQL|PostgreSQL|NATS>`. Refer to the Chart's available values for more information on the supported options.

The following example shows how to install PostgreSQL as the alternative default datastore for Kamaji.


Use the makefiles under `deploy/kine/postgresql` to deploy the proper resources (e.g. `deployment`, `certificates` and `secret`). For the sake of this example, we'll override the variable `NAME` to create the resources so they match the sample manifest used in the next step.

```bash
make -C ./deploy/kine/postgresql/ postgresql NAME=gold
```

When all the resources are ready, apply the following sample chart:

```bash
kubectl apply -f ./config/samples/kamaji_v1alpha1_datastore_postgresql_gold.yaml 
```

Check the `Datastore` creation:

```bash
kubectl get datastores
NAME                 DRIVER       READY   AGE
postgresql-gold   PostgreSQL              18s
```

The `Datastore` stays not ready until the Kamaji chart is installed, since no operator is yet running to reconcile it. Install it with:

```bash
helm install kamaji ./charts/kamaji -n kamaji-system --create-namespace \
  --set kamaji-etcd.deploy=false \
  --set datastore.driver=PostgreSQL \
  --set datastore.endpoints[0]=postgres-gold-rw.postgres-system.svc:5432 \
  --set datastore.basicAuth.usernameSecret.name=postgres-gold-superuser \
  --set datastore.basicAuth.usernameSecret.namespace=postgres-system \
  --set datastore.basicAuth.usernameSecret.keyPath=username \
  --set datastore.basicAuth.passwordSecret.name=postgres-gold-superuser \
  --set datastore.basicAuth.passwordSecret.namespace=postgres-system \
  --set datastore.basicAuth.passwordSecret.keyPath=password \
  --set datastore.tlsConfig.certificateAuthority.certificate.name=postgres-gold-ca \
  --set datastore.tlsConfig.certificateAuthority.certificate.namespace=postgres-system \
  --set datastore.tlsConfig.certificateAuthority.certificate.keyPath=ca.crt \
  --set datastore.tlsConfig.certificateAuthority.privateKey.name=postgres-gold-ca \
  --set datastore.tlsConfig.certificateAuthority.privateKey.namespace=postgres-system \
  --set datastore.tlsConfig.certificateAuthority.privateKey.keyPath=ca.key \
  --set datastore.tlsConfig.clientCertificate.certificate.name=postgres-gold-root-cert \
  --set datastore.tlsConfig.clientCertificate.certificate.namespace=postgres-system \
  --set datastore.tlsConfig.clientCertificate.certificate.keyPath=tls.crt \
  --set datastore.tlsConfig.clientCertificate.privateKey.name=postgres-gold-root-cert \
  --set datastore.tlsConfig.clientCertificate.privateKey.namespace=postgres-system \
  --set datastore.tlsConfig.clientCertificate.privateKey.keyPath=tls.key
```

Once the operator is fully deployed, the `Datastore` resource should appear in a `Ready` state.

```bash
NAME                 DRIVER       READY   AGE
postgresql-gold   PostgreSQL   true    4m40s
```

Once the installation is complete, you can create Tenant Control Planes that use the alternative default datastore.

Apply a `TenantControlPlane` manifest:

```bash
cat > test-tenant-gold.yaml <<EOF 
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: k8s-133
  labels:
    tenant.clastix.io: k8s-133
spec:
  dataStore: postgresql-gold #this should match the Datastore's resource NAME
  controlPlane:
    deployment:
      replicas: 2
    service:
      serviceType: LoadBalancer
  kubernetes:
    version: "v1.33.0"
    kubelet:
      configurationJSONPatches:
        - op: add
          path: /featureGates
          value:
            KubeletCrashLoopBackOffMax: false
            KubeletEnsureSecretPulledImages: false
        - op: replace
          path: /cgroupDriver
          value: systemd
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
EOF

kubectl apply -f test-tenant-gold.yaml
```

Finally, clean up the resources:
```bash
kubectl delete -f test-tenant-gold.yaml
kubectl delete -f ./config/samples/kamaji_v1alpha1_datastore_postgresql_gold.yaml 
make -C ./deploy/kine/postgresql postgresql-destroy
```


## Defining specific Datastore per Tenant Control Plane

Each `TenantControlPlane` can refer to a specific `Datastore` thanks to the `/spec/dataStore` field.
This allows you to implement your preferred sharding or pooling strategy. 

When this key is omitted, Kamaji will use the default datastore configured with its CLI argument `--datastore`.

The following example shows how to use MySQL as an alternative datastore for each TenantControlPlane.

Install Kamaji disabling the default datastore through:

```bash
helm install kamaji ./charts/kamaji -n kamaji-system --create-namespace --set kamaji-etcd.deploy=false
```

Use the makefiles under `deploy/kine/mysql` to deploy the proper resources (e.g. `deployment`, `certificates` and `secret`). For the sake of this example, we'll override the variable `NAME` to create the resources so they match the sample manifest used in the next step.

```bash
make -C ./deploy/kine/mysql mariadb NAME=gold
```

Then, apply the sample `DataStore` manifest:

```bash
kubectl apply -f ./config/samples/kamaji_v1alpha1_datastore_mysql_gold.yaml
```

Check the created datastore with:

```bash
kubectl get datastores

NAME         DRIVER   READY   AGE
mysql-gold   MySQL    true    30s
```

Apply the `TenantControlPlane` manifest:

```bash
cat > test-tenant-gold.yaml <<EOF 
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: k8s-133
  labels:
    tenant.clastix.io: k8s-133
spec:
  dataStore: mysql-gold #this should match the Datastore's resource NAME
  controlPlane:
    deployment:
      replicas: 2
    service:
      serviceType: LoadBalancer
  kubernetes:
    version: "v1.33.0"
    kubelet:
      configurationJSONPatches:
        - op: add
          path: /featureGates
          value:
            KubeletCrashLoopBackOffMax: false
            KubeletEnsureSecretPulledImages: false
        - op: replace
          path: /cgroupDriver
          value: systemd
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
EOF

kubectl apply -f test-tenant-gold.yaml
```

Check the `TenantControlPlane` through:

```bash
kubectl get tcp -A 
NAMESPACE   NAME      VERSION   INSTALLED VERSION   STATUS   CONTROL-PLANE ENDPOINT   KUBECONFIG                 DATASTORE    AGE
default     k8s-133   v1.33.0   v1.33.0             Ready    10.10.10.200:6443        k8s-133-admin-kubeconfig   mysql-gold   43s
```

Finally, cleanup the resources:
```bash
kubectl delete -f test-tenant-gold.yaml
kubectl delete -f ./config/samples/kamaji_v1alpha1_datastore_mysql_gold.yaml 
make -C ./deploy/kine/mysql mariadb-destroy NAME=gold
```

## NATS considerations

The NATS support is still experimental, mostly because multi-tenancy is **NOT** supported.

A `NATS` based DataStore can host one and only one Tenant Control Plane. When a `TenantControlPlane` refers to a NATS `DataStore` already used by another instance, its reconciliation will fail and be blocked.
