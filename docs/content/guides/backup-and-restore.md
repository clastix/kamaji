# Backup and Restore

As mentioned in the introduction, Tenant Control Planes are just regular pods scheduled in the Management Cluster. As such, you can take advantage of the same backup and restore methods that you would use to maintain the standard workload.

This guide will assist you in how to backup and restore TCP resources on the Management Cluster using [Velero](https://tanzu.vmware.com/developer/guides/what-is-velero/).

## Prerequisites

Before proceeding with the next steps, we assume that the following prerequisites are met:

- Working Kamaji setup
- Working datastore resource
- Working TCP resource
- Velero binary installed on the operator VM
- Velero installed on the Management Cluster
- Configured BackupStorageLocation for Velero

## Backup step

This example shows how to backup and restore a Tenant Control Plane called `tenant-00` and related resources using the `--include-namespaces` tag. Assume the Tenant Control Plane is deployed into the `tenant-00` namespace:

```
velero backup create tenant-00 --include-namespaces tenant-00
```

then, verify the backup job status:

```
velero backup get tenant-00
NAME        STATUS     ERRORS   WARNINGS   CREATED                         EXPIRES   STORAGE LOCATION   SELECTOR
tenant-00   Completed  0        0          2023-02-23 17:45:13 +0100 CET   27d       cloudian           <none>
```

in case of problems, you can get more information by running:

```
velero backup describe tenant-00
```

## Restore step

!!! warning "Restoring Datastore"
    This procedure will restore just the TCP resource.
    
    In the event that the related datastore has been lost, you MUST restore it BEFORE continue; to do this, refer to the backup and restore strategy of the datastore of your choice.

To restore just the desired TCP, simply execute:

```
velero restore create tenant-00 \
    --from-backup tenant-00 \
    --include-resources tcp,secret \
    --status-include-resources tcp
```

verify the restore job status:

```
velero restore get

NAME           BACKUP         STATUS      STARTED                         COMPLETED                       ERRORS   WARNINGS   CREATED                         SELECTOR
tenant-00      tenant-00      Completed   2023-02-24 12:31:39 +0100 CET   2023-02-24 12:31:40 +0100 CET   0        0          2023-02-24 12:31:39 +0100 CET   <none>
```

In a bunch of seconds, the Kamaji controller will reconcile the TCP and its status will pass from Ready, to NotReady and, finally, Ready again:

```
kubectl get tcp -A

NAMESPACE   NAME           VERSION   STATUS   CONTROL-PLANE ENDPOINT   KUBECONFIG                      DATASTORE   AGE
tenant-00   solar-energy   v1.25.6   Ready    192.168.1.251:8443       solar-energy-admin-kubeconfig   dedicated   6m
[...]
```
