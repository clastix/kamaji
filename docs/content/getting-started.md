# Getting started

This document explains how to deploy a minimal Kamaji setup on [KinD](https://kind.sigs.k8s.io/) for development scopes. Please refer to the [Kamaji documentation](concepts.md) for understanding all the terms used in this guide, as for example: `admin cluster`, `tenant cluster`, and `tenant control plane`.

## Pre-requisites

We assume you have installed on your workstation:

- [Docker](https://docker.com)
- [KinD](https://kind.sigs.k8s.io/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
- [kubeadm](https://kubernetes.io/docs/tasks/tools/#kubeadm)
- [Helm](https://helm.sh/docs/intro/install/)
- [jq](https://stedolan.github.io/jq/)
- [openssl](https://www.openssl.org/)
- [cfssl/cfssljson](https://github.com/cloudflare/cfssl)


> Starting from Kamaji v0.1.0, `kubectl` and `kubeadm` need to meet at least minimum version to `v1.25.0` due to the changes regarding the `kubelet-config` ConfigMap required for the node join.

## Setup Kamaji on KinD

The instance of Kamaji is made of a single node hosting:

- admin control-plane
- admin worker
- multi-tenant datastore

### Standard Installation

You can install your KinD cluster, an `etcd` based multi-tenant datastore and the Kamaji operator with a **single command**:

```bash
$ make -C deploy/kind
```

Now you can deploy a [`TenantControlPlane`](#deploy-tenant-control-plane).

### Installation with alternative datastore drivers

Kamaji offers the possibility of using a different storage system than `etcd` for datastore, like `MySQL` or `PostgreSQL` compatible databases.

First, setup a KinD cluster and the other requirements:

```bash
$ make -C deploy/kind reqs
```

Install one of the alternative supported databases:

- **MySQL** install it with command:

    `$ make -C deploy/kine/mysql mariadb`

- **PostgreSQL** install it with command:

    `$ make -C deploy/kine/postgresql postgresql`

Then use Helm to install the Kamaji Operator and make sure it uses a datastore with the proper driver `datastore.driver=<MySQL|PostgreSQL>`.

For example, with a PostreSQL datastore:

```bash
helm install kamaji charts/kamaji -n kamaji-system --create-namespace \
  --set etcd.deploy=false \
  --set datastore.driver=PostgreSQL \
  --set datastore.endpoints[0]=postgres-default-rw.kamaji-system.svc:5432 \
  --set datastore.basicAuth.usernameSecret.name=postgres-default-superuser \
  --set datastore.basicAuth.usernameSecret.namespace=kamaji-system \
  --set datastore.basicAuth.usernameSecret.keyPath=username \
  --set datastore.basicAuth.passwordSecret.name=postgres-default-superuser \
  --set datastore.basicAuth.passwordSecret.namespace=kamaji-system \
  --set datastore.basicAuth.passwordSecret.keyPath=password \
  --set datastore.tlsConfig.certificateAuthority.certificate.name=postgres-default-ca \
  --set datastore.tlsConfig.certificateAuthority.certificate.namespace=kamaji-system \
  --set datastore.tlsConfig.certificateAuthority.certificate.keyPath=ca.crt \
  --set datastore.tlsConfig.certificateAuthority.privateKey.name=postgres-default-ca \
  --set datastore.tlsConfig.certificateAuthority.privateKey.namespace=kamaji-system \
  --set datastore.tlsConfig.certificateAuthority.privateKey.keyPath=ca.key \
  --set datastore.tlsConfig.clientCertificate.certificate.name=postgres-default-root-cert \
  --set datastore.tlsConfig.clientCertificate.certificate.namespace=kamaji-system \
  --set datastore.tlsConfig.clientCertificate.certificate.keyPath=tls.crt \
  --set datastore.tlsConfig.clientCertificate.privateKey.name=postgres-default-root-cert \
  --set datastore.tlsConfig.clientCertificate.privateKey.namespace=kamaji-system \
  --set datastore.tlsConfig.clientCertificate.privateKey.keyPath=tls.key
```

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

### Get the kubeconfig

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
