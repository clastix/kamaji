# Datastore Migration

On the Management Cluster, you can deploy one or more multi-tenant datastores as `etcd`, `PostgreSQL`, `MySQL`, and `NATS` to save the state of the Tenant Clusters.
A Tenant Control Plane can be migrated from a datastore to another one without service disruption or without complex and error-prone backup & restore procedures.

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
# kubectl get datastore dedicated -o yaml
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

Migration is expected to complete in 5 minutes.
However, that timeout can be customized at the `TenantControlPlane` level with the annotation `kamaji.clastix.io/migration-timeout` with a Go-duration value (e.g.: `5m`).

!!! info "Leftover"
    Please, note the datastore migration leaves the data on the default datastore, so you have to remove it manually.

!!! info "Avoiding stale DataStore content"
    When migrating `TenantControlPlane` across DataStore, a collision with the __schema__ name could happen,
    leading to unexpected results such as old data still available.
    The annotation `kamaji.clastix.io/cleanup-prior-migration=true` allows to enforce the clean-up of the target `DataStore` schema in case of collision.

## Post migration
After migrating data to the new datastore, complete the migration procedure by restarting the `kubelet.service` on all the tenant worker nodes.

## Troubleshooting

### Migration Job Image Version
When migrating between datastores, the Kamaji controller automatically creates a migration job to transfer data from the source to the destination datastore. By default, this job uses the same image version as the running Kamaji controller. If you need to use a different image version for the migration job, you can specify it by passing extra arguments to the controller:

```shell
helm upgrade kamaji clastix/kamaji --version ${CHART_VERSION} -n kamaji-system 
--set extraArgs[0]=--migrate-image=custom/kamaji:version`
```

### Handling Private Registry Images
If the Kamaji controller images are stored in a private registry that requires authentication, the migration job will fail because it does not use any `ImagePullSecret` by default. You need to attach your registry secret to the `kamaji-controller-manager` service account, which is used by the migration job. You can do this with the following command:

```shell
kubectl -n kamaji-system patch serviceaccount kamaji-controller-manager \
        -p '{"imagePullSecrets": [{"name": "myregistry-credentials"}]}'
```

This command patches the kamaji-controller-manager service account to include your registry secret, allowing the migration job to pull images from the private registry successfully.

### Running under restricted PodSecurity

If the `kamaji-system` namespace enforces a `restricted` [Pod Security Standard](https://kubernetes.io/docs/concepts/security/pod-security-standards/), three components need a compliant `securityContext`: the datastore migration Job, the kine sidecar (non-etcd datastores), and the Konnectivity agent.

#### Migration Job

The migration Job always runs with a fixed, `restricted`-compliant `securityContext`, so it is admissible under any Pod Security Standard without configuration:

```yaml
securityContext:            # pod
  runAsNonRoot: true
  seccompProfile:
    type: RuntimeDefault
containers:
  - securityContext:        # container
      allowPrivilegeEscalation: false
      capabilities:
        drop:
          - ALL
```

The Job only copies datastore data and never needs privileges, so these values are not configurable.

!!! warning "Custom migration images"
    No `runAsUser` is enforced, which means the image's own user applies. The default migration image is the Kamaji controller image, which runs as a non-root user. If you override it with `--migrate-image`, that image must run as non-root as well — otherwise the kubelet rejects the container with `container has runAsNonRoot and image will run as root`.

#### Kine sidecar (non-etcd datastores)

For `MySQL`, `PostgreSQL`, or other non-etcd datastores the control plane pod includes a `kine` sidecar and a `kine-init` chmod init container. Set their security contexts via `spec.controlPlane.deployment.containerSecurityContexts`:

```yaml
spec:
  controlPlane:
    deployment:
      containerSecurityContexts:
        kine:
          allowPrivilegeEscalation: false
          runAsNonRoot: true
          capabilities:
            drop:
              - ALL
          seccompProfile:
            type: RuntimeDefault
        kineInit:
          allowPrivilegeEscalation: false
          runAsNonRoot: true
          capabilities:
            drop:
              - ALL
          seccompProfile:
            type: RuntimeDefault
```

!!! tip
    If your datastore image runs as root by default, add an explicit `runAsUser` (e.g. `runAsUser: 1000`) matching a non-root UID in the image.

#### Konnectivity agent

When the Konnectivity addon is enabled, set the agent container's security context via `spec.addons.konnectivity.agent.securityContext`:

```yaml
spec:
  addons:
    konnectivity:
      agent:
        securityContext:
          allowPrivilegeEscalation: false
          runAsNonRoot: true
          capabilities:
            drop:
              - ALL
          seccompProfile:
            type: RuntimeDefault
```

!!! tip
    Same note applies: if the `proxy-agent` image runs as root, add `runAsUser` with a non-root UID.

