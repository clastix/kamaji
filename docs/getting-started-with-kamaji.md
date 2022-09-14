# Setup a minimal Kamaji for development

This document explains how to deploy a minimal Kamaji setup on [KinD](https://kind.sigs.k8s.io/) for development scopes. Please refer to the [Kamaji documentation](../concepts.md) for understanding all the terms used in this guide, as for example: `admin cluster`, `tenant cluster`, and `tenant control plane`.

## Pre-requisites

We assume you have installed on your workstation:

- [Docker](https://docs.docker.com/engine/install/)
- [KinD](https://kind.sigs.k8s.io/)
- [kubectl@v1.25.0](https://kubernetes.io/docs/tasks/tools/)
- [kubeadm@v1.25.0](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/)
- [jq](https://stedolan.github.io/jq/)
- [openssl](https://www.openssl.org/)
- [cfssl](https://github.com/cloudflare/cfssl)
- [cfssljson](https://github.com/cloudflare/cfssl)


> Starting from Kamaji v0.0.2, `kubectl` and `kubeadm` need to meet at least minimum version to `v1.25.0`:
> this is required due to the latest changes addressed from the release Kubernetes 1.25 release regarding the `kubelet-config` ConfigMap required for the node join.

## Setup Kamaji on KinD

The instance of Kamaji is made of a single node hosting:

- admin control-plane
- admin worker
- multi-tenant datastore

### Standard installation

You can install your KinD cluster, ETCD multi-tenant cluster and Kamaji operator with a **single command**:

```bash
$ make -C deploy/kind
```

Now you can [create your first `TenantControlPlane`](#deploy-tenant-control-plane).

### Data store-specific

Kamaji offers the possibility of using a different storage system than `ETCD` for the tenants, like `MySQL` or `PostgreSQL` compatible databases.

First, setup a KinD cluster:

```bash
$ make -C deploy/kind kind
```

#### ETCD

Deploy a multi-tenant `ETCD` cluster into the Kamaji node:

```bash
$ make -C deploy/kind etcd-cluster
```

Now you're ready to [install Kamaji operator](#install-kamaji).

#### MySQL

Deploy a MySQL/MariaDB backend into the Kamaji node:

```bash
$ make -C deploy/kine/mysql mariadb
```

Adjust the [Kamaji install manifest](../config/install.yaml) according to the example of a [MySQL DataStore](../config/samples/kamaji_v1alpha1_datastore_mysql.yaml) and make sure Kamaji uses the proper datastore name:

```
--datastore={.metadata.name}
```

Now you're ready to [install Kamaji operator](#install-kamaji).

#### PostgreSQL

Deploy a PostgreSQL backend into the Kamaji node:

```bash
$ make -C deploy/kine/postgresql postgresql
```

Adjust the [Kamaji install manifest](../config/install.yaml) according to the example of a [PostgreSQL DataStore](../config/samples/kamaji_v1alpha1_datastore_postgresql.yaml) and make sure Kamaji uses the proper datastore name:

```
--datastore={.metadata.name}
```

Now you're ready to [install Kamaji operator](#install-kamaji).

### Install Kamaji

```bash
$ kubectl apply -f config/install.yaml
```

> If you experience some errors during the apply of the manifest as `resource mapping not found ... ensure CRDs are installed first`, just apply it again.

### Deploy Tenant Control Plane

Now it is the moment of deploying your first tenant control plane.

```bash
$ kubectl apply -f - <<EOF
apiVersion: kamaji.clastix.io/v1alpha1
kind: TenantControlPlane
metadata:
  name: tenant1
spec:
  controlPlane:
    deployment:
      replicas: 2
      additionalMetadata:
        annotations:
          environment.clastix.io: tenant1
          tier.clastix.io: "0"
        labels:
          tenant.clastix.io: tenant1
          kind.clastix.io: deployment
    service:
      additionalMetadata:
        annotations:
          environment.clastix.io: tenant1
          tier.clastix.io: "0"
        labels:
          tenant.clastix.io: tenant1
          kind.clastix.io: service
      serviceType: NodePort
  kubernetes:
    version: "v1.23.4"
    kubelet:
      cgroupfs: cgroupfs
    admissionControllers:
    - LimitRanger
    - ResourceQuota
  networkProfile:
    address: "172.18.0.2"
    port: 31443
    certSANs:
    - "test.clastixlabs.io"
    serviceCidr: "10.96.0.0/16"
    podCidr: "10.244.0.0/16"
    dnsServiceIPs: 
    - "10.96.0.10"
  addons:
    coreDNS: {}
    kubeProxy: {}
EOF
```

> Check networkProfile fields according to your installation
> To let Kamaji works in kind, you have indicate that the service must be [NodePort](https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport)

### Get Kubeconfig

Let's retrieve kubeconfig and store in `/tmp/kubeconfig`

```bash
$ kubectl get secrets tenant1-admin-kubeconfig -o json \
 | jq -r '.data["admin.conf"]' \
 | base64 -d > /tmp/kubeconfig
 ```

It can be export it, to facilitate the next tasks:

```bash
$ export KUBECONFIG=/tmp/kubeconfig
```

### Install CNI

We highly recommend to install [kindnet](https://github.com/aojea/kindnet) as CNI for your kamaji TCP.

```bash
$ kubectl create -f https://raw.githubusercontent.com/aojea/kindnet/master/install-kindnet.yaml
```

### Join worker nodes

```bash
$ make -C deploy/kind kamaji-kind-worker-join
```

> To add more worker nodes, run again the command above.

Check out the node:

```bash
$ kubectl get nodes
NAME           STATUS   ROLES    AGE   VERSION
d2d4b468c9de   Ready    <none>   44s   v1.23.4
```

> For more complex scenarios (exposing port, different version and so on), run `join-node.bash`.

Tenant control plane provision has been finished in a minimal Kamaji setup based on KinD. Therefore, you could develop, test and make your own experiments with Kamaji.

## Cleanup

```bash
$ make -C deploy/kind destroy
```
