# Datastore Migration

On the admin cluster, you can deploy one or more multi-tenant datastores as `etcd`, `PostgreSQL`, and `MySQL` to save the state of the tenant clusters. A Tenant Control Plane can be migrated from a datastore to another one without service disruption or without complex and error prone backup & restore procedures.

This guide will assist you to live migrate Tenant's data from a datastore to another one having the same `etcd` driver.

## Prerequisites

Assume you have a Tenant Control Plane using the default datastore:

``` shell
kubectl get tcp
NAME        VERSION   STATUS   CONTROL-PLANE ENDPOINT   KUBECONFIG                   DATASTORE   AGE
tenant-00   v1.25.2   Ready    192.168.32.200:6443      tenant-00-admin-kubeconfig   default     8d
```

You can check a custom resource called `DataStore` providing a declarative description of the `default` datastore:

```yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: DataStore
metadata:
  annotations:
  labels:
  name: default
spec:
  driver: etcd
  endpoints:
  - etcd-0.etcd.kamaji-system.svc.cluster.local:2379
  - etcd-1.etcd.kamaji-system.svc.cluster.local:2379
  - etcd-2.etcd.kamaji-system.svc.cluster.local:2379
  tlsConfig:
    certificateAuthority:
      certificate:
        secretReference:
          keyPath: ca.crt
          name: etcd-certs
          namespace: kamaji-system
      privateKey:
        secretReference:
          keyPath: ca.key
          name: etcd-certs
          namespace: kamaji-system
    clientCertificate:
      certificate:
        secretReference:
          keyPath: tls.crt
          name: etcd-root-client-certs
          namespace: kamaji-system
      privateKey:
        secretReference:
          keyPath: tls.key
          name: etcd-root-client-certs
          namespace: kamaji-system
status:
  usedBy:
  - default/tenant-00
```

The `default` datastore is installed by Kamaji Helm chart in the same namespace hosting the controller:

```shell
kubectl -n kamaji-system get pods
NAME                              READY   STATUS      RESTARTS   AGE
etcd-0                            1/1     Running     0          23d
etcd-1                            1/1     Running     0          23d
etcd-2                            1/1     Running     0          23d
kamaji-5d6cdfbbb9-bn27f           1/1     Running     0          2d19h
```

## Install a new datastore
A managed datastore is highly recommended in production. The [kamaji-etcd](https://github.com/clastix/kamaji-etcd) project provides a viable option to setup a managed multi-tenant `etcd` running as StatefulSet made of three replicas:

```bash
helm repo add clastix https://clastix.github.io/charts
helm repo update
helm install dedicated clastix/kamaji-etcd -n dedicated --create-namespace --set datastore.enabled=true
```

You should end up with a new datastore `dedicated` provided by an `etcd` cluster:

```yaml
kubectl get datastore dedicated -o yaml
apiVersion: kamaji.clastix.io/v1alpha1
kind: DataStore
metadata:
  annotations:
  labels:
  name: dedicated
spec:
  driver: etcd
  endpoints:
  - dedicated-0.dedicated.dedicated.svc.cluster.local:2379
  - dedicated-1.dedicated.dedicated.svc.cluster.local:2379
  - dedicated-2.dedicated.dedicated.svc.cluster.local:2379
  tlsConfig:
    certificateAuthority:
      certificate:
        secretReference:
          keyPath: ca.crt
          name: dedicated-certs
          namespace: dedicated
      privateKey:
        secretReference:
          keyPath: ca.key
          name: dedicated-certs
          namespace: dedicated
    clientCertificate:
      certificate:
        secretReference:
          keyPath: tls.crt
          name: dedicated-root-client-certs
          namespace: dedicated
      privateKey:
        secretReference:
          keyPath: tls.key
          name: dedicated-root-client-certs
          namespace: dedicated
status: {}
```

Check the `etcd` cluster:

```bash
kubectl -n dedicated get sts,pods,pvc
NAME                         READY   AGE
statefulset.apps/dedicated   3/3     25h

NAME                                  READY   STATUS      RESTARTS   AGE
pod/dedicated-0                       1/1     Running     0          25h
pod/dedicated-1                       1/1     Running     0          25h
pod/dedicated-2                       1/1     Running     0          25h

NAME                                     STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
persistentvolumeclaim/data-dedicated-0   Bound    pvc-a5c66737-ef78-4689-b863-037f8382ed78   10Gi       RWO            local-path     25h
persistentvolumeclaim/data-dedicated-1   Bound    pvc-1e9f77eb-89f3-4256-9508-c18b71fca7ea   10Gi       RWO            local-path     25h
persistentvolumeclaim/data-dedicated-2   Bound    pvc-957c4802-1e7c-4f37-ac01-b89ad1fa9fdb   10Gi       RWO            local-path     25h
```

## Migrate data
To migrate data from current `default` datastore to the new dedicated one, patch the Tenant Control Plane `tenant-00` to use the new `dedicated` datastore:

```shell
kubectl patch --type merge tcp tenant-00 -p '{"spec": {"dataStore": "dedicated"}}'
```

and check the process happening in real time:

```shell
kubectl get tcp -w
NAME        VERSION   STATUS      CONTROL-PLANE ENDPOINT   KUBECONFIG                   DATASTORE   AGE
tenant-00   v1.25.2   Ready       192.168.32.200:6443      tenant-00-admin-kubeconfig   default     9d
tenant-00   v1.25.2   Migrating   192.168.32.200:6443      tenant-00-admin-kubeconfig   default     9d
tenant-00   v1.25.2   Migrating   192.168.32.200:6443      tenant-00-admin-kubeconfig   default     9d
tenant-00   v1.25.2   Migrating   192.168.32.200:6443      tenant-00-admin-kubeconfig   dedicated   9d
tenant-00   v1.25.2   Migrating   192.168.32.200:6443      tenant-00-admin-kubeconfig   dedicated   9d
tenant-00   v1.25.2   Ready       192.168.32.200:6443      tenant-00-admin-kubeconfig   dedicated   9d
```

During the datastore migration, the Tenant Control Plane is put in read-only mode to avoid misalignments between source and destination datastores. If tenant users try to update the data, an admission controller denies the request with the following message:


```shell
Error from server (the current Control Plane is in freezing mode due to a maintenance mode,
all the changes are blocked: removing the webhook may lead to an inconsistent state upon its completion):
admission webhook "catchall.migrate.kamaji.clastix.io" denied the request
```

After a while, depending on the amount of data to migrate, the Tenant Control Plane is put back in full operating mode by the Kamaji controller.

> Please, note the datastore migration leaves the data on the default datastore, so you have to remove it manually.
